/*
Copyright 2020-2021 The Kubernetes Authors.

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

package rules

import (
	"fmt"

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/pci"
)

type PciIDRule struct {
	source.MatchExpressionSet
}

// Match PCI devices on provided PCI device attributes
func (r *PciIDRule) Match() (bool, error) {
	devs, ok := source.GetFeatureSource("pci").GetFeatures().Instances[pci.DeviceFeature]
	if !ok {
		return false, fmt.Errorf("cpuid information not available")
	}

	return r.MatchInstances(devs.Features)
}
