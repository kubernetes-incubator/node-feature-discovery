/*
Copyright 2020 The Kubernetes Authors.

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

package custom

import (
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/rules"
)

// getStaticFeatures returns statically configured custom features to discover
// e.g RMDA related features. NFD configuration file may extend these custom features by adding rules.
func getStaticFeatureConfig() []FeatureSpec {
	return []FeatureSpec{
		{
			Name: "rdma.capable",
			MatchOn: []LegacyRule{
				{
					PciID: &rules.PciIDRule{
						source.MatchExpressionSet{
							"vendor": source.NewMatchExpression(source.MatchIn, "15b3"),
						},
					},
				},
			},
		},
		{
			Name: "rdma.available",
			MatchOn: []LegacyRule{
				{
					LoadedKMod: &rules.LoadedKModRule{
						source.MatchExpressionSet{
							"ib_uverbs": source.NewMatchExpression(source.MatchExists),
							"rdma_ucm":  source.NewMatchExpression(source.MatchExists),
						},
					},
				},
			},
		},
	}
}
