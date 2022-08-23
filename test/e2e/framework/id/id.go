package id

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Id struct {
	gvr  schema.GroupVersionResource
	ns   string
	name string
}

func New(gvr schema.GroupVersionResource, ns, name string) Id { return Id{gvr, ns, name} }
func (r Id) GetGvr() schema.GroupVersionResource              { return r.gvr }
func (r Id) GetNamespace() string                             { return r.ns }
func (r Id) GetName() string                                  { return r.name }
func (r Id) IsClustered() bool                                { return r.ns == "" }
func (r Id) IsNamespaced() bool                               { return !r.IsClustered() }
