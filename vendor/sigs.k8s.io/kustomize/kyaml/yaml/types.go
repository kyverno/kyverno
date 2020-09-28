// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/kyaml/errors"
	"sigs.k8s.io/kustomize/kyaml/sets"
)

// CopyYNode returns a distinct copy of its argument.
// Use https://github.com/jinzhu/copier instead?
func CopyYNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := *n
	if len(n.Content) > 0 {
		// Using Go 'copy' here doesn't yield independent slices.
		c.Content = make([]*Node, len(n.Content))
		for i, item := range n.Content {
			c.Content[i] = CopyYNode(item)
		}
	}
	return &c
}

// IsYNodeTaggedNull returns true if the node is explicitly tagged Null.
func IsYNodeTaggedNull(n *yaml.Node) bool {
	return n != nil && n.Tag == NodeTagNull
}

// IsYNodeEmptyMap is true if the Node is a non-nil empty map.
func IsYNodeEmptyMap(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.MappingNode && len(n.Content) == 0
}

// IsYNodeEmptyMap is true if the Node is a non-nil empty sequence.
func IsYNodeEmptySeq(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.SequenceNode && len(n.Content) == 0
}

// IsYNodeEmptyDoc is true if the node is a Document with no content.
// E.g.: "---\n---"
func IsYNodeEmptyDoc(n *yaml.Node) bool {
	return n.Kind == yaml.DocumentNode && n.Content[0].Tag == NodeTagNull
}

func IsYNodeString(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == NodeTagString
}

// Parser parses values into configuration.
type Parser struct {
	Kind  string `yaml:"kind,omitempty"`
	Value string `yaml:"value,omitempty"`
}

func (p Parser) Filter(_ *RNode) (*RNode, error) {
	d := yaml.NewDecoder(bytes.NewBuffer([]byte(p.Value)))
	o := &RNode{value: &yaml.Node{}}
	return o, d.Decode(o.value)
}

// TODO(pwittrock): test this
func GetStyle(styles ...string) Style {
	var style Style
	for _, s := range styles {
		switch s {
		case "TaggedStyle":
			style |= TaggedStyle
		case "DoubleQuotedStyle":
			style |= DoubleQuotedStyle
		case "SingleQuotedStyle":
			style |= SingleQuotedStyle
		case "LiteralStyle":
			style |= LiteralStyle
		case "FoldedStyle":
			style |= FoldedStyle
		case "FlowStyle":
			style |= FlowStyle
		}
	}
	return style
}

// Filter defines a function to manipulate an individual RNode such as by changing
// its values, or returning a field.
//
// When possible, Filters should be serializable to yaml so that they can be described
// declaratively as data.
//
// Analogous to http://www.linfo.org/filters.html
type Filter interface {
	Filter(object *RNode) (*RNode, error)
}

type FilterFunc func(object *RNode) (*RNode, error)

func (f FilterFunc) Filter(object *RNode) (*RNode, error) {
	return f(object)
}

// TypeMeta partially copies apimachinery/pkg/apis/meta/v1.TypeMeta
// No need for a direct dependence; the fields are stable.
type TypeMeta struct {
	// APIVersion is the apiVersion field of a Resource
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	// Kind is the kind field of a Resource
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
}

// Hardcoded list.
// TODO(#2861): replace this with data acquired from openapi.
var notNamespaceableKinds = []string{
	"APIService",
	"CSIDriver",
	"CSINode",
	"CertificateSigningRequest",
	"Cluster",
	"ClusterRole",
	"ClusterRoleBinding",
	"ComponentStatus",
	"CustomResourceDefinition",
	"MutatingWebhookConfiguration",
	"Namespace",
	"Node",
	"PersistentVolume",
	"PodSecurityPolicy",
	"PriorityClass",
	"RuntimeClass",
	"SelfSubjectAccessReview",
	"SelfSubjectRulesReview",
	"StorageClass",
	"SubjectAccessReview",
	"TokenReview",
	"ValidatingWebhookConfiguration",
	"VolumeAttachment",
}

// IsNamespaceable returns true if this TypeMeta is for an object
// that can be placed in a namespace.
// Implements https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#not-all-objects-are-in-a-namespace
func (tm TypeMeta) IsNamespaceable() bool {
	for _, k := range notNamespaceableKinds {
		if k == tm.Kind {
			return false
		}
	}
	return true
}

// NameMeta contains name information.
type NameMeta struct {
	// Name is the metadata.name field of a Resource
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Namespace is the metadata.namespace field of a Resource
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// ResourceMeta contains the metadata for a both Resource Type and Resource.
type ResourceMeta struct {
	TypeMeta `json:",inline" yaml:",inline"`
	// ObjectMeta is the metadata field of a Resource
	ObjectMeta `yaml:"metadata,omitempty"`
}

// ObjectMeta contains metadata about a Resource
type ObjectMeta struct {
	NameMeta `json:",inline" yaml:",inline"`
	// Labels is the metadata.labels field of a Resource
	Labels map[string]string `yaml:"labels,omitempty"`
	// Annotations is the metadata.annotations field of a Resource.
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// GetIdentifier returns a ResourceIdentifier that includes
// the information needed to uniquely identify a resource in a cluster.
func (m *ResourceMeta) GetIdentifier() ResourceIdentifier {
	return ResourceIdentifier{
		TypeMeta: m.TypeMeta,
		NameMeta: m.NameMeta,
	}
}

// ResourceIdentifier contains the information needed to uniquely
// identify a resource in a cluster.
type ResourceIdentifier struct {
	TypeMeta `json:",inline" yaml:",inline"`
	NameMeta `json:",inline" yaml:",inline"`
}

// Comments struct is comment yaml comment types
type Comments struct {
	LineComment string `yaml:"lineComment,omitempty"`
	HeadComment string `yaml:"headComment,omitempty"`
	FootComment string `yaml:"footComment,omitempty"`
}

func (r *ResourceIdentifier) GetName() string {
	return r.Name
}

func (r *ResourceIdentifier) GetNamespace() string {
	return r.Namespace
}

func (r *ResourceIdentifier) GetAPIVersion() string {
	return r.APIVersion
}

func (r *ResourceIdentifier) GetKind() string {
	return r.Kind
}

const (
	Trim = "Trim"
	Flow = "Flow"
)

// String returns a string value for a Node, applying the supplied formatting options
func String(node *yaml.Node, opts ...string) (string, error) {
	if node == nil {
		return "", nil
	}
	optsSet := sets.String{}
	optsSet.Insert(opts...)
	if optsSet.Has(Flow) {
		oldStyle := node.Style
		defer func() {
			node.Style = oldStyle
		}()
		node.Style = yaml.FlowStyle
	}

	b := &bytes.Buffer{}
	e := NewEncoder(b)
	err := e.Encode(node)
	e.Close()
	val := b.String()
	if optsSet.Has(Trim) {
		val = strings.TrimSpace(val)
	}
	return val, errors.Wrap(err)
}
