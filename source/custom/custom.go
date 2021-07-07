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
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"

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

	nameTemplate *template.Template
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
	for i, spec := range *conf {
		if strings.Contains(spec.Name, "{{") {
			(*conf)[i].nameTemplate = template.Must(template.New("").Option("missingkey=error").Parse(spec.Name))
		}
	}

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
		ruleOut, err := spec.discover(domainFeatures)
		if err != nil {
			klog.Errorf("failed to discover feature: %q: %s", spec.Name, err.Error())
			continue
		}
		for k, v := range ruleOut {
			labels[k] = v
		}
	}
	return labels, nil
}

// Process a single feature by Matching on the defined rules.
func (s *FeatureSpec) discover(features map[string]*feature.DomainFeatures) (map[string]string, error) {
	ret := make(map[string]string)

	if len(s.MatchOn) > 0 {
		// Logical OR over the legacy rules
		matched := false
		for _, matchRule := range s.MatchOn {
			if m, err := matchRule.match(); err != nil {
				return nil, err
			} else if m {
				matched = true

				if err := s.expandName(nil, ret); err != nil {
					return nil, err
				}

				break
			}
		}
		if !matched {
			return nil, nil
		}
	}

	if len(s.MatchAny) > 0 {
		// Logical OR over the matchAny domains rules
		matched := false
		for _, matchRule := range s.MatchAny {
			if m, err := matchRule.match(features); err != nil {
				return nil, err
			} else if m != nil {
				matched = true
				utils.KlogDump(4, "matches for matchAny "+s.Name, "  ", m)

				if err := s.expandName(m, ret); err != nil {
					return nil, err
				}

				if s.nameTemplate == nil {
					// No templating so we stop here (further matches would just
					// produce the same labels)
					break
				}
			}
		}
		if !matched {
			return nil, nil
		}
	}

	if len(s.MatchAll) > 0 {
		// Logical AND over the matchAny domains rules
		for _, matchRule := range s.MatchAll {
			if m, err := matchRule.match(features); err != nil {
				return nil, err
			} else if m == nil {
				return nil, nil
			} else {

				utils.KlogDump(4, "matches for matchAll "+s.Name, "  ", m)
				if err := s.expandName(m, ret); err != nil {
					return nil, err
				}
			}
		}
	}

	// We have a match
	return ret, nil
}

func (s *FeatureSpec) expandName(in matchedFeatures, out map[string]string) error {
	expandedName := s.Name
	if s.nameTemplate != nil {
		// Execute template to produce an array of labels
		var tmp bytes.Buffer
		if err := s.nameTemplate.Execute(&tmp, in); err != nil {
			return err
		}
		expandedName = tmp.String()
	}

	// Split out individual labels
	for _, item := range strings.Split(expandedName, "\n") {
		// Remove leading/trailing whitespace and skip empty lines
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			n, v := s.getNameValue(trimmed)
			out[n] = v
		}
	}
	return nil
}

func (s *FeatureSpec) getNameValue(name string) (string, string) {
	// Value can be overridden in Name with "key=value". This is useful for
	// templates.
	if strings.ContainsRune(name, '=') {
		split := strings.SplitN(name, "=", 2)
		return split[0], split[1]
	}

	if s.Value != nil {
		return name, *s.Value
	}

	return name, "true"
}

type matchedFeatures map[string]domainMatchedFeatures

type domainMatchedFeatures map[string]interface{}

func (r *MatchRule) match(features map[string]*feature.DomainFeatures) (matchedFeatures, error) {
	ret := make(matchedFeatures, len(*r))

	for key, rules := range *r {
		split := strings.SplitN(key, ".", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("invalid rule %q: must be <domain>.<feature>", key)
		}
		domain := split[0]
		// Ignore case
		featureName := strings.ToLower(split[1])

		domainFeatures, ok := features[domain]
		if !ok {
			return nil, fmt.Errorf("unknown feature source/domain %q", domain)
		}

		if _, ok := ret[domain]; !ok {
			ret[domain] = make(domainMatchedFeatures)
		}

		var m bool
		var e error
		if f, ok := domainFeatures.Keys[featureName]; ok {
			v, err := rules.MatchGetKeys(f.Features)
			m = len(v) > 0
			e = err
			ret[domain][featureName] = v
		} else if f, ok := domainFeatures.Values[featureName]; ok {
			v, err := rules.MatchGetValues(f.Features)
			m = len(v) > 0
			e = err
			ret[domain][featureName] = v
		} else if f, ok := domainFeatures.Instances[featureName]; ok {
			v, err := rules.MatchGetInstances(f.Features)
			m = len(v) > 0
			e = err
			ret[domain][featureName] = v
		} else {
			return nil, fmt.Errorf("%q feature of source/domain %q not available", featureName, domain)
		}

		if e != nil {
			return nil, e
		} else if !m {
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
