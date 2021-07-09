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

package custom

import (
	"reflect"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/rules"
)

const Name = "custom"

// Legacy rules
type LegacyRule struct {
	PciID      *rules.PciIDRule      `json:"pciId,omitempty"`
	UsbID      *rules.UsbIDRule      `json:"usbId,omitempty"`
	LoadedKMod *rules.LoadedKModRule `json:"loadedKMod,omitempty"`
	CpuID      *rules.CpuIDRule      `json:"cpuId,omitempty"`
	Kconfig    *rules.KconfigRule    `json:"kConfig,omitempty"`
	Nodename   *rules.NodenameRule   `json:"nodename,omitempty"`
}

type FeatureSpec struct {
	nfdv1alpha1.Rule

	MatchOn []LegacyRule `json:"matchOn"`
}

type config []FeatureSpec

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *config {
	return &config{}
}

// customSource implements the LabelSource and ConfigurableSource interfaces.
type customSource struct {
	config *config
}

type Rule interface {
	// Match on rule
	Match() (bool, error)
}

// Singleton source instance
var (
	src customSource
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns the name of the feature source
func (s *customSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *customSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *customSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *customSource) SetConfig(c source.Config) {
	switch c.(type) {
	case *config:
	default:
		klog.Fatalf("invalid config type: %T", c)
	}

	// Parse template rules
	conf := c.(*config)
	s.config = conf
}

// Priority method of the LabelSource interface
func (s *customSource) Priority() int { return 10 }

// GetLabels method of the LabelSource interface
func (s *customSource) GetLabels() (source.FeatureLabels, error) {
	// Get raw features from all sources
	domainFeatures := make(map[string]*feature.DomainFeatures)
	for n, s := range source.GetAllFeatureSources() {
		domainFeatures[n] = s.GetFeatures()
	}

	labels := source.FeatureLabels{}
	allFeatureConfig := append(getStaticFeatureConfig(), *s.config...)
	allFeatureConfig = append(allFeatureConfig, getDirectoryFeatureConfig()...)
	utils.KlogDump(2, "custom features configuration:", "  ", allFeatureConfig)
	// Iterate over features
	for _, spec := range allFeatureConfig {
		ruleOut, err := spec.Match(domainFeatures)
		if err != nil {
			klog.Errorf("failed to discover feature: %q: %s", spec.Name, err.Error())
			continue
		}
		if !spec.NoLabel {
			for k, v := range ruleOut {
				labels[k] = v
			}
		}
		// Feed back rule output to features map for subsequent rules to match
		feature.InsertValues(domainFeatures, nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut)
	}
	return labels, nil
}

// Process a single feature by Matching on the defined rules.
func (s *FeatureSpec) Match(features map[string]*feature.DomainFeatures) (map[string]string, error) {
	ret, err := s.Rule.Match(features)
	if err != nil {
		return nil, err
	} else if ret == nil {
		// No match
		return nil, err
	}

	if len(s.MatchOn) > 0 {
		// Logical OR over the legacy rules
		matched := false
		for _, matchRule := range s.MatchOn {
			if m, err := matchRule.match(); err != nil {
				return nil, err
			} else if m {
				matched = true

				// Only expand if no matchAny/matchAll rules were run
				if len(ret) == 0 {
					if err := s.Rule.ExpandName(nil, ret); err != nil {
						return nil, err
					}
				}

				break
			}
		}
		if !matched {
			return nil, nil
		}
	}

	return ret, nil
}

func (r *LegacyRule) match() (bool, error) {
	allRules := []Rule{
		r.PciID,
		r.UsbID,
		r.LoadedKMod,
		r.CpuID,
		r.Kconfig,
		r.Nodename,
	}

	// return true, nil if all rules match
	matchRules := func(rules []Rule) (bool, error) {
		for _, rule := range rules {
			if reflect.ValueOf(rule).IsNil() {
				continue
			}
			if match, err := rule.Match(); err != nil {
				return false, err
			} else if !match {
				return false, nil
			}
		}
		return true, nil
	}

	return matchRules(allRules)
}

func init() {
	source.Register(&src)
}
