package framework

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
)

func TestPodUpdateAdmissionRequest_CarriesBothObjects(t *testing.T) {
	// Policies that compare oldObject with object only work if the UPDATE request carries both,
	// each in its own field. Swapping them would turn every immutability test into a no-op.
	oldRaw := []byte(`{"metadata":{"labels":{"team":"payments"}}}`)
	newRaw := []byte(`{"metadata":{"labels":{"team":"search"}}}`)

	req := PodUpdateAdmissionRequest("billing-api", "default", oldRaw, newRaw)

	assert.Equal(t, admissionv1.Update, req.Operation)
	assert.Equal(t, newRaw, req.Object.Raw, "object must be the incoming pod")
	assert.Equal(t, oldRaw, req.OldObject.Raw, "oldObject must be the stored pod")
	assert.Equal(t, "billing-api", req.Name)
	assert.Equal(t, "default", req.Namespace)
	assert.Equal(t, "Pod", req.Kind.Kind)
	assert.Equal(t, "pods", req.Resource.Resource)
	require.NotNil(t, req.RequestResource, "the ivpol engine reads RequestResource to pick image extractors")
	assert.Equal(t, req.Resource, *req.RequestResource)
	assert.NotEmpty(t, req.UID)
}

func TestPodUpdateAdmissionRequest_UniqueUIDs(t *testing.T) {
	// The handlers key reports and events on the request UID, so two requests must never share one.
	raw := []byte(`{}`)
	a := PodUpdateAdmissionRequest("p", "default", raw, raw)
	b := PodUpdateAdmissionRequest("p", "default", raw, raw)
	assert.NotEqual(t, a.UID, b.UID)
}
