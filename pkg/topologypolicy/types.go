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

package topologypolicy

import (
	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/apis/config"
)

// TopologyManagerPolicy constants which represent the current configuration
// for Topology manager policy and Topology manager scope in Kubelet config
type TopologyManagerPolicy string

const (
	SingleNumaContainerScope TopologyManagerPolicy = TopologyManagerPolicy(v1alpha1.SingleNUMANodeContainerLevel)
	SingleNumaPodScope       TopologyManagerPolicy = TopologyManagerPolicy(v1alpha1.SingleNUMANodePodLevel)
	Restricted               TopologyManagerPolicy = TopologyManagerPolicy(v1alpha1.Restricted)
	BestEffort               TopologyManagerPolicy = TopologyManagerPolicy(v1alpha1.BestEffort)
	None                     TopologyManagerPolicy = TopologyManagerPolicy(v1alpha1.None)
)

// K8sTopologyPolicies are resource allocation policies constants
type K8sTopologyManagerPolicies string

const (
	singleNumaNode K8sTopologyManagerPolicies = config.SingleNumaNodeTopologyManagerPolicy
	restricted     K8sTopologyManagerPolicies = config.RestrictedTopologyManagerPolicy
	bestEffort     K8sTopologyManagerPolicies = config.BestEffortTopologyManagerPolicy
	none           K8sTopologyManagerPolicies = config.NoneTopologyManagerPolicy
)

// K8sTopologyScopes are constants which defines the granularity
// at which you would like resource alignment to be performed.
type K8sTopologyManagerScopes string

const (
	pod       K8sTopologyManagerScopes = config.PodTopologyManagerScope
	container K8sTopologyManagerScopes = config.ContainerTopologyManagerScope
)
