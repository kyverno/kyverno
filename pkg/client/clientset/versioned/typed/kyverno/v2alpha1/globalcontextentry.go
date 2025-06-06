/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v2alpha1

import (
	context "context"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	scheme "github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// GlobalContextEntriesGetter has a method to return a GlobalContextEntryInterface.
// A group's client should implement this interface.
type GlobalContextEntriesGetter interface {
	GlobalContextEntries() GlobalContextEntryInterface
}

// GlobalContextEntryInterface has methods to work with GlobalContextEntry resources.
type GlobalContextEntryInterface interface {
	Create(ctx context.Context, globalContextEntry *kyvernov2alpha1.GlobalContextEntry, opts v1.CreateOptions) (*kyvernov2alpha1.GlobalContextEntry, error)
	Update(ctx context.Context, globalContextEntry *kyvernov2alpha1.GlobalContextEntry, opts v1.UpdateOptions) (*kyvernov2alpha1.GlobalContextEntry, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, globalContextEntry *kyvernov2alpha1.GlobalContextEntry, opts v1.UpdateOptions) (*kyvernov2alpha1.GlobalContextEntry, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*kyvernov2alpha1.GlobalContextEntry, error)
	List(ctx context.Context, opts v1.ListOptions) (*kyvernov2alpha1.GlobalContextEntryList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *kyvernov2alpha1.GlobalContextEntry, err error)
	GlobalContextEntryExpansion
}

// globalContextEntries implements GlobalContextEntryInterface
type globalContextEntries struct {
	*gentype.ClientWithList[*kyvernov2alpha1.GlobalContextEntry, *kyvernov2alpha1.GlobalContextEntryList]
}

// newGlobalContextEntries returns a GlobalContextEntries
func newGlobalContextEntries(c *KyvernoV2alpha1Client) *globalContextEntries {
	return &globalContextEntries{
		gentype.NewClientWithList[*kyvernov2alpha1.GlobalContextEntry, *kyvernov2alpha1.GlobalContextEntryList](
			"globalcontextentries",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *kyvernov2alpha1.GlobalContextEntry { return &kyvernov2alpha1.GlobalContextEntry{} },
			func() *kyvernov2alpha1.GlobalContextEntryList { return &kyvernov2alpha1.GlobalContextEntryList{} },
		),
	}
}
