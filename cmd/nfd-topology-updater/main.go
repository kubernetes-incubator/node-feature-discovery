/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/dumpobject"
	"sigs.k8s.io/node-feature-discovery/pkg/kubeconf"

	topology "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/podres"

	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	"sigs.k8s.io/node-feature-discovery/pkg/topologypolicy"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-topology-updater"
)

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	printVersion := flags.Bool("version", false, "Print version and exit.")

	args, resourcemonitorArgs := parseArgs(flags, os.Args[1:]...)

	if *printVersion {
		fmt.Println(ProgramName, version.Get())
		os.Exit(0)
	}

	// Assert that the version is known
	if version.Undefined() {
		klog.Warningf("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	// Plug klog into grpc logging infrastructure
	utils.ConfigureGrpcKlog()

	klConfig, err := kubeconf.GetKubeletConfigFromLocalFile(resourcemonitorArgs.KubeletConfigFile)
	if err != nil {
		klog.Fatalf("error getting topology Manager Policy: %v", err)
	}
	tmPolicy := string(topologypolicy.DetectTopologyPolicy(klConfig.TopologyManagerPolicy, klConfig.TopologyManagerScope))
	klog.Infof("Detected kubelet Topology Manager policy %q", tmPolicy)

	podResClient, err := podres.GetPodResClient(resourcemonitorArgs.PodResourceSocketPath)
	if err != nil {
		klog.Fatalf("Failed to get PodResource Client: %v", err)
	}
	var resScan resourcemonitor.ResourcesScanner

	resScan, err = resourcemonitor.NewPodResourcesScanner(resourcemonitorArgs.Namespace, podResClient)
	if err != nil {
		klog.Fatalf("Failed to initialize ResourceMonitor instance: %v", err)
	}

	// CAUTION: these resources are expected to change rarely - if ever.
	// So we are intentionally do this once during the process lifecycle.
	// TODO: Obtain node resources dynamically from the podresource API
	zonesChannel := make(chan v1alpha1.ZoneList)
	var zones v1alpha1.ZoneList

	resAggr, err := resourcemonitor.NewResourcesAggregator(resourcemonitorArgs.SysfsRoot, podResClient)
	if err != nil {
		klog.Fatalf("Failed to obtain node resource information: %v", err)
	}

	klog.Infof("resAggr is: %v\n", resAggr)
	go func() {
		for {
			klog.Infof("Scanning\n")

			podResources, err := resScan.Scan()
			klog.Infof("podResources are: %v\n", podResources)
			if err != nil {
				klog.Warningf("Scan failed: %v\n", err)
				continue
			}

			zones = resAggr.Aggregate(podResources)
			zonesChannel <- zones
			klog.Infof("After aggregating resources identified zones are:%v", dumpobject.DumpObject(zones))

			time.Sleep(resourcemonitorArgs.SleepInterval)
		}
	}()

	// Get new TopologyUpdater instance
	instance, err := topology.NewTopologyUpdater(*args, tmPolicy)
	if err != nil {
		klog.Exitf("failed to initialize TopologyUpdater instance: %v", err)
	}
	for {

		zonesValue := <-zonesChannel
		klog.Infof("Received value on ZoneChannel\n")
		if err = instance.Update(zonesValue); err != nil {
			klog.Exit(err)
		}
		if args.Oneshot {
			break
		}
	}

}

func parseArgs(flags *flag.FlagSet, osArgs ...string) (*topology.Args, *resourcemonitor.Args) {
	args, resourcemonitorArgs := initFlags(flags)

	_ = flags.Parse(osArgs)
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	return args, resourcemonitorArgs
}

func initFlags(flagset *flag.FlagSet) (*topology.Args, *resourcemonitor.Args) {
	args := &topology.Args{}
	resourcemonitorArgs := &resourcemonitor.Args{}

	flagset.StringVar(&args.CaFile, "ca-file", "",
		"Root certificate for verifying connections")
	flagset.StringVar(&args.CertFile, "cert-file", "",
		"Certificate used for authenticating connections")
	flagset.StringVar(&args.KeyFile, "key-file", "",
		"Private key matching -cert-file")
	flagset.BoolVar(&args.Oneshot, "oneshot", false,
		"Update once and exit")
	flagset.BoolVar(&args.NoPublish, "no-publish", false,
		"Do not publish discovered features to the cluster-local Kubernetes API server.")
	flagset.DurationVar(&resourcemonitorArgs.SleepInterval, "sleep-interval", time.Duration(60)*time.Second,
		"Time to sleep between CR updates. Non- positive value implies no re-labeling (i.e. infinite sleep). [Default: 60s]")
	flagset.StringVar(&resourcemonitorArgs.Namespace, "watch-namespace", "",
		"Namespace to watch pods. Use \"\" for all namespaces.")
	flagset.StringVar(&resourcemonitorArgs.KubeletConfigFile, "kubelet-config-file", "/podresources/config.yaml",
		"Kubelet config file path.")
	flagset.StringVar(&resourcemonitorArgs.PodResourceSocketPath, "podresources-socket", "/podresources/kubelet.sock",
		"Pod Resource Socket path to use.")
	flagset.StringVar(&resourcemonitorArgs.SysfsRoot, "sysfs", "/host",
		"Mount point of the sysfs.")
	flagset.StringVar(&args.Server, "server", "localhost:8080",
		"NFD server address to connecto to.")
	flagset.StringVar(&args.ServerNameOverride, "server-name-override", "",
		"Hostname expected from server certificate, useful in testing")

	initKlogFlags(flagset)

	return args, resourcemonitorArgs
}

func initKlogFlags(flagset *flag.FlagSet) {

	flags := flag.NewFlagSet("klog flags", flag.ExitOnError)
	//flags.SetOutput(ioutil.Discard)
	klog.InitFlags(flags)

}
