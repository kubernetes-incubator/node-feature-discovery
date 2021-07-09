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

package v1alpha1

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

// Match exercises a Rule against a set of features.
func (r *Rule) Match(features feature.Features) (map[string]string, error) {
	ret := make(map[string]string)

	if len(r.MatchAny) > 0 {
		// Logical OR over the matchAny domains rules
		matched := false
		for _, matchRule := range r.MatchAny {
			if m, err := matchRule.match(features); err != nil {
				return nil, err
			} else if m != nil {
				matched = true
				utils.KlogDump(4, "matches for matchAny "+r.Name, "  ", m)

				if err := r.ExpandName(m, ret); err != nil {
					return nil, err
				}

				if isTemplate, err := r.NameIsTemplate(); err != nil {
					return nil, nil
				} else if !isTemplate {
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

	if len(r.MatchAll) > 0 {
		// Logical AND over the matchAny domains rules
		for _, matchRule := range r.MatchAll {
			if m, err := matchRule.match(features); err != nil {
				return nil, err
			} else if m == nil {
				return nil, nil
			} else {
				utils.KlogDump(4, "matches for matchAll "+r.Name, "  ", m)
				if err := r.ExpandName(m, ret); err != nil {
					return nil, err
				}
			}
		}
	}

	// We have a match
	return ret, nil
}

func (r *Rule) ExpandName(in matchedFeatures, out map[string]string) error {
	expandedName, err := r.executeNameTemplate(in)
	if err != nil {
		return nil
	}

	// Split out individual labels
	for _, item := range strings.Split(expandedName, "\n") {
		// Remove leading/trailing whitespace and skip empty lines
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			n, v := r.getNameValue(trimmed)
			out[n] = v
		}
	}
	return nil
}

func (r *Rule) getNameValue(name string) (string, string) {
	// Value can be overridden in Name with "key=value". This is useful for
	// templates.
	if strings.ContainsRune(name, '=') {
		split := strings.SplitN(name, "=", 2)
		return split[0], split[1]
	}

	if r.Value != nil {
		return name, *r.Value
	}

	return name, "true"
}

func (r *Rule) executeNameTemplate(data interface{}) (string, error) {
	n, err := r.getNameTemplateHelper()
	if err != nil {
		return "", err
	}
	return n.expand(data)
}

func (r *Rule) NameIsTemplate() (bool, error) {
	n, err := r.getNameTemplateHelper()
	if err != nil {
		return true, err
	}
	return n.nameTemplate != nil, nil
}

type matchedFeatures map[string]domainMatchedFeatures

type domainMatchedFeatures map[string]interface{}

func (r *MatchRule) match(features feature.Features) (matchedFeatures, error) {
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

func (r *Rule) getNameTemplateHelper() (*nameTemplateHelper, error) {
	if r.name == nil {
		n, err := newNameTemplateHelper(r.Name)
		if err != nil {
			return nil, err
		}
		r.name = n
	}
	return r.name, nil
}

type nameTemplateHelper struct {
	name         string
	nameTemplate *template.Template
}

func newNameTemplateHelper(name string) (*nameTemplateHelper, error) {
	e := nameTemplateHelper{name: name}

	if strings.Contains(name, "{{") {

		tmpl, err := template.New("").Option("missingkey=error").Parse(name)
		if err != nil {
			return nil, fmt.Errorf("invalid template in rule name: %w", err)
		}
		e.nameTemplate = tmpl
	}
	return &e, nil
}

// DeepCopy is a stub to augment the auto-generated code
func (in *nameTemplateHelper) DeepCopy() *nameTemplateHelper {
	if in == nil {
		return nil
	}
	out := new(nameTemplateHelper)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a stub to augment the auto-generated code
func (in *nameTemplateHelper) DeepCopyInto(out *nameTemplateHelper) {
	// HACK: just re-use the template
	out.nameTemplate = in.nameTemplate
}

func (e *nameTemplateHelper) expand(data interface{}) (string, error) {
	if e.nameTemplate == nil {
		return e.name, nil
	}

	var tmp bytes.Buffer
	if err := e.nameTemplate.Execute(&tmp, data); err != nil {
		return "", err
	}
	return tmp.String(), nil
}
