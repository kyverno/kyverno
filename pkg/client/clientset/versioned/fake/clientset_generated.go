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

package fake

import (
	clientset "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1/fake"
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	fakekyvernov1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2/fake"
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	fakekyvernov1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1/fake"
	kyvernov2alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2alpha1"
	fakekyvernov2alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2alpha1/fake"
	kyvernov2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	fakekyvernov2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1/fake"
	wgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	fakewgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

// NewSimpleClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewSimpleClientset(objects ...runtime.Object) *Clientset {
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &Clientset{tracker: o}
	cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *Clientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

var (
	_ clientset.Interface = &Clientset{}
	_ testing.FakeClient  = &Clientset{}
)

// KyvernoV1 retrieves the KyvernoV1Client
func (c *Clientset) KyvernoV1() kyvernov1.KyvernoV1Interface {
	return &fakekyvernov1.FakeKyvernoV1{Fake: &c.Fake}
}

// KyvernoV1alpha2 retrieves the KyvernoV1alpha2Client
func (c *Clientset) KyvernoV1alpha2() kyvernov1alpha2.KyvernoV1alpha2Interface {
	return &fakekyvernov1alpha2.FakeKyvernoV1alpha2{Fake: &c.Fake}
}

// KyvernoV1beta1 retrieves the KyvernoV1beta1Client
func (c *Clientset) KyvernoV1beta1() kyvernov1beta1.KyvernoV1beta1Interface {
	return &fakekyvernov1beta1.FakeKyvernoV1beta1{Fake: &c.Fake}
}

// KyvernoV2beta1 retrieves the KyvernoV2beta1Client
func (c *Clientset) KyvernoV2beta1() kyvernov2beta1.KyvernoV2beta1Interface {
	return &fakekyvernov2beta1.FakeKyvernoV2beta1{Fake: &c.Fake}
}

// KyvernoV2alpha1 retrieves the KyvernoV2alpha1Client
func (c *Clientset) KyvernoV2alpha1() kyvernov2alpha1.KyvernoV2alpha1Interface {
	return &fakekyvernov2alpha1.FakeKyvernoV2alpha1{Fake: &c.Fake}
}

// Wgpolicyk8sV1alpha2 retrieves the Wgpolicyk8sV1alpha2Client
func (c *Clientset) Wgpolicyk8sV1alpha2() wgpolicyk8sv1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &fakewgpolicyk8sv1alpha2.FakeWgpolicyk8sV1alpha2{Fake: &c.Fake}
}
