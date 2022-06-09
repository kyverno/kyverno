package resource

import (
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type Resource struct {
	gvr  schema.GroupVersionResource
	ns   string
	data []byte
}

func (r *Resource) Gvr() schema.GroupVersionResource { return r.gvr }
func (r *Resource) Namespace() string                { return r.ns }
func (r *Resource) Data() []byte                     { return r.data }
func (r *Resource) IsClustered() bool                { return r.ns == "" }
func (r *Resource) IsNamespaced() bool               { return !r.IsClustered() }

func (r *Resource) Unstructured() *unstructured.Unstructured {
	var u unstructured.Unstructured
	gomega.Expect(yaml.Unmarshal(r.data, &u)).To(gomega.Succeed())
	// TODO: set namespace ?
	// TODO: ensure GV(R/K) ?
	return &u
}

func Resources(resources ...Resource) []Resource {
	return resources
}
