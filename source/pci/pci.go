/*
Copyright 2018-2021 The Kubernetes Authors.

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

package pci

import (
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "pci"

const DeviceFeature = "device"

type Config struct {
	DeviceClassWhitelist []string `json:"deviceClassWhitelist,omitempty"`
	DeviceLabelFields    []string `json:"deviceLabelFields,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		DeviceClassWhitelist: []string{"03", "0b40", "12"},
		DeviceLabelFields:    []string{"class", "vendor"},
	}
}

// pciSource implements the FeatureSource, LabelSource and ConfigurableSource interfaces.
type pciSource struct {
	config   *Config
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src pciSource
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns the name of the feature source
func (s *pciSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *pciSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *pciSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *pciSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
}

// Priority method of the LabelSource interface
func (s *pciSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *pciSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}

	// Construct a device label format, a sorted list of valid attributes
	deviceLabelFields := make([]string, 0)
	configLabelFields := make(map[string]struct{}, len(s.config.DeviceLabelFields))
	for _, field := range s.config.DeviceLabelFields {
		configLabelFields[field] = struct{}{}
	}

	for _, attr := range mandatoryDevAttrs {
		if _, ok := configLabelFields[attr]; ok {
			deviceLabelFields = append(deviceLabelFields, attr)
			delete(configLabelFields, attr)
		}
	}
	if len(configLabelFields) > 0 {
		keys := []string{}
		for key := range configLabelFields {
			keys = append(keys, key)
		}
		klog.Warningf("invalid fields (%s) in deviceLabelFields, ignoring...", strings.Join(keys, ", "))
	}
	if len(deviceLabelFields) == 0 {
		klog.Warningf("no valid fields in deviceLabelFields defined, using the defaults")
		deviceLabelFields = []string{"class", "vendor"}
	}

	// Iterate over all device classes
	for _, dev := range s.features.Instances[DeviceFeature].Features {
		attrs := dev.Attributes
		class := attrs["class"]
		for _, white := range s.config.DeviceClassWhitelist {
			if strings.HasPrefix(string(class), strings.ToLower(white)) {
				devLabel := ""
				for i, attr := range deviceLabelFields {
					devLabel += attrs[attr]
					if i < len(deviceLabelFields)-1 {
						devLabel += "_"
					}
				}
				labels[devLabel+".present"] = true

				if _, ok := attrs["sriov_totalvfs"]; ok {
					labels[devLabel+".sriov.capable"] = true
				}
				break
			}
		}
	}
	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *pciSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	devs, err := detectPci()
	if err != nil {
		return fmt.Errorf("failed to detect PCI devices: %s", err.Error())
	}
	s.features.Instances[DeviceFeature] = feature.InstanceFeatures{Features: devs}

	utils.KlogDump(3, "discovered pci features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *pciSource) GetFeatures() *feature.DomainFeatures {
	return s.features
}

func init() {
	source.Register(&src)
}
