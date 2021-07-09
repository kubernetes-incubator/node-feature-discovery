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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelRuleList contains a list of LabelRule objects.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LabelRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LabelRule `json:"items"`
}

// LabelRule resource specifies a configuration for custom node labeling.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
type LabelRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LabelRuleSpec `json:"spec"`
}

// RuleSpec defines a set of labeling rules.
type LabelRuleSpec struct {
	Rules []Rule `json:"rules"`
}

// Rule defines a rule for creating on feature label.
type Rule struct {
	// Name of the label to be generated.
	Name string `json:"name"`

	// Value of the label, optional.
	// +optional
	Value *string `json:"value,omitempty"`

	// NoLabel disables the label output of the rule. Output value(s) of the
	// matching process are only added as values under a special "MATCHES" key
	// for subsequent rules to use as input features for matching.
	// +optional
	NoLabel bool `json:"noLabel"`

	// MatchAny specifies a list of expression sets one of which must match
	// +optional
	MatchAny []MatchRule `json:"matchAny"`

	// MatchAll specifies a list of expression sets all of which must match
	// +optional
	MatchAll []MatchRule `json:"matchAll"`

	// name is a private helper/cache for handling golang templates
	name *nameTemplateHelper `json:"-"`
}

// MatchRule specifies a set of expressions for one feature.
type MatchRule map[string]FeatureRule

// FeatureRule defines rules for one feature, matching against its attributes.
type FeatureRule struct {
	MatchExpressionSet `json:",inline"`
}

// MatchExpressionSet contains a set of MatchExpressions, each of which is
// evaluated against a set of input values.
type MatchExpressionSet struct {
	Expressions `json:",inline"`
}

// Expressions is a helper type to work around issues with k8s deepcopy-gen
type Expressions map[string]*MatchExpression

// MatchExpression specifies an expression to evaluate against a set of input
// values. It contains an operator that is applied when matching the input and
// an array of values that the operator evaluates the input against.
type MatchExpression struct {
	// Op is the operator to be applied.
	Op MatchOp `json:"op"`

	// Value is the list of values that the operand evaluates the input
	// against. Value should empty if the operator is Exists, DoesNotExist,
	// IsTrue or IsFalse.  Value should contain a single element if the
	// operator is Gt or Lt. In other cases Value should contain at least one
	// element.
	// +optional
	Value MatchValue `json:"value,omitempty"`
}

// MatchOp is the match operator that is applied on values when evaluating a
// MatchExpression.
// +kubebuilder:validation:Enum="In";"NotIn";"InRegexp";"Exists";"DoesNotExist";"Gt";"Lt";"IsTrue";"IsFalse"
type MatchOp string

// MatchValue is the list of values associated with a MatchExpression.
type MatchValue []string

const (
	MatchAny          MatchOp = ""
	MatchIn           MatchOp = "In"
	MatchNotIn        MatchOp = "NotIn"
	MatchInRegexp     MatchOp = "InRegexp"
	MatchExists       MatchOp = "Exists"
	MatchDoesNotExist MatchOp = "DoesNotExist"
	MatchGt           MatchOp = "Gt"
	MatchLt           MatchOp = "Lt"
	MatchIsTrue       MatchOp = "IsTrue"
	MatchIsFalse      MatchOp = "IsFalse"
)

// MatchAllNames is a special key in MatchExpressionSet to use field names
// (keys from the input) instead of values when matching.
const MatchAllNames = "*"

const (
	// RuleBackrefDomain is the special feature domain for backreferencing
	// output of preceding rules.
	RuleBackrefDomain = "rule"
	// RuleBackrefFeature is the special feature name for backreferencing
	// output of preceding rules.
	RuleBackrefFeature = "matched"
)
