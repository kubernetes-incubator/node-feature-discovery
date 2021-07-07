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
	"fmt"
	"reflect"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/rules"
)

const Name = "custom"

type MatchRule map[string]source.MatchExpressionSet

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
	Name     string       `json:"name"`
	Value    *string      `json:"value,omitempty"`
	MatchOn  []LegacyRule `json:"matchOn"`
	MatchAny []MatchRule  `json:"matchAny"`
	MatchAll []MatchRule  `json:"matchAll"`
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
func (s *customSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
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
	for _, customFeature := range allFeatureConfig {
		featureExist, err := customFeature.discover(domainFeatures)
		if err != nil {
			klog.Errorf("failed to discover feature: %q: %s", customFeature.Name, err.Error())
			continue
		}
		if featureExist {
			var value interface{} = true
			if customFeature.Value != nil {
				value = *customFeature.Value
			}
			labels[customFeature.Name] = value
		}
	}
	return labels, nil
}

// Process a single feature by Matching on the defined rules.
func (s *FeatureSpec) discover(features map[string]*feature.DomainFeatures) (bool, error) {
	if len(s.MatchOn) > 0 {
		// Logical OR over the legacy rules
		matched := false
		for _, matchRule := range s.MatchOn {
			if match, err := matchRule.match(); err != nil {
				return false, err
			} else if match {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
	}

	if len(s.MatchAny) > 0 {
		// Logical OR over the matchAny rules
		matched := false
		for _, matchRule := range s.MatchAny {
			if match, err := matchRule.match(features); err != nil {
				return false, err
			} else if match {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
	}

	if len(s.MatchAll) > 0 {
		// Logical AND over the matchAll rules
		for _, matchRule := range s.MatchAll {
			if match, err := matchRule.match(features); err != nil {
				return false, err
			} else if !match {
				return false, nil
			}
		}
	}

	return true, nil
}

func (r *MatchRule) match(features map[string]*feature.DomainFeatures) (bool, error) {
	for key, rules := range *r {
		split := strings.SplitN(key, ".", 2)
		if len(split) != 2 {
			return false, fmt.Errorf("invalid rule %q: must be <domain>.<feature>", key)
		}
		domain := split[0]
		// Ignore case
		featureName := strings.ToLower(split[1])

		domainFeatures, ok := features[domain]
		if !ok {
			return false, fmt.Errorf("unknown feature source/domain %q", domain)
		}

		var m bool
		var err error
		if f, ok := domainFeatures.Keys[featureName]; ok {
			m, err = rules.MatchKeys(f.Features)
		} else if f, ok := domainFeatures.Values[featureName]; ok {
			m, err = rules.MatchValues(f.Features)
		} else if f, ok := domainFeatures.Instances[featureName]; ok {
			m, err = rules.MatchInstances(f.Features)
		} else {
			return false, fmt.Errorf("%q feature of source/domain %q not available", featureName, domain)
		}

		if err != nil {
			return false, err
		} else if !m {
			return false, nil
		}
	}
	return true, nil
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
