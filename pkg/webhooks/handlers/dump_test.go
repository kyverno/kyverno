package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func Test_RedactPayload(t *testing.T) {
	tc := []struct {
		name           string
		requestPayload []byte
	}{
		{
			name: "request payload with nil old object",
			requestPayload: []byte(`{
				"uid":"631a230b-b949-468d-b9ae-927fdd76217e",
				"kind":{
					"group":"",
					"version":"v1",
					"kind":"Secret"
				},
				"resource":{
					"group":"",
					"version":"v1",
					"resource":"secrets"
				},
				"requestKind":{
					"group":"",
					"version":"v1",
					"kind":"Secret"
				},
				"requestResource":{
					"group":"",
					"version":"v1",
					"resource":"secrets"
				},
				"name":"mysecret2",
				"namespace":"default",
				"operation":"CREATE",
				"userInfo":{
					"username":"kubernetes-admin",
					"groups":["system:masters","system:authenticated"]
				},
				"object":{
					"kind":"Secret",
					"apiVersion":"v1",
					"metadata":{
						"name":"mysecret2",
						"namespace":"default",
						"uid":"de6f1564-295d-4c57-a10b-f37358414a81",
						"creationTimestamp":"2022-10-20T15:17:56Z",
						"labels":{
							"purpose":"production"
						},
						"annotations":{
							"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"data\":{\"password\":\"MWYyZDFlMmU2N2Rm\",\"username\":\"YWRtaW4=\"},\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"labels\":{\"purpose\":\"production\"},\"name\":\"mysecret2\",\"namespace\":\"default\"}}\n"},"managedFields":[{"manager":"kubectl-client-side-apply","operation":"Update","apiVersion":"v1","time":"2022-10-20T15:17:56Z","fieldsType":"FieldsV1","fieldsV1":{"f:data":{".":{},"f:password":{},"f:username":{}},"f:metadata":{"f:annotations":{".":{},"f:kubectl.kubernetes.io/last-applied-configuration":{}},"f:labels":{".":{},"f:purpose":{}}},"f:type":{}}}]},
					"data":{
						"password":"MWYyZDFlMmU2N2Rm",
						"username":"YWRtaW4="
					},
					"type":"Opaque"
				},
				"oldObject":null,
				"dryRun":false,
				"options":{
					"kind":"CreateOptions",
					"apiVersion":"meta.k8s.io/v1",
					"fieldManager":"kubectl-client-side-apply",
					"fieldValidation":"Strict"
				}
			}`),
		},
		{
			name: "request payload with non nil old object",
			requestPayload: []byte(`{
				"uid":"631a230b-b949-468d-b9ae-927fdd76217e",
				"kind":{
					"group":"",
					"version":"v1",
					"kind":"Secret"
				},
				"resource":{
					"group":"",
					"version":"v1",
					"resource":"secrets"
				},
				"requestKind":{
					"group":"",
					"version":"v1",
					"kind":"Secret"
				},
				"requestResource":{
					"group":"",
					"version":"v1",
					"resource":"secrets"
				},
				"name":"mysecret2",
				"namespace":"default",
				"operation":"CREATE",
				"userInfo":{
					"username":"kubernetes-admin",
					"groups":["system:masters","system:authenticated"]
				},
				"object": null,
				"oldObject":{
					"kind":"Secret",
					"apiVersion":"v1",
					"metadata":{
						"name":"mysecret2",
						"namespace":"default",
						"uid":"de6f1564-295d-4c57-a10b-f37358414a81",
						"creationTimestamp":"2022-10-20T15:17:56Z",
						"labels":{
							"purpose":"production"
						},
						"annotations":{
							"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"data\":{\"password\":\"MWYyZDFlMmU2N2Rm\",\"username\":\"YWRtaW4=\"},\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"labels\":{\"purpose\":\"production\"},\"name\":\"mysecret2\",\"namespace\":\"default\"}}\n"},"managedFields":[{"manager":"kubectl-client-side-apply","operation":"Update","apiVersion":"v1","time":"2022-10-20T15:17:56Z","fieldsType":"FieldsV1","fieldsV1":{"f:data":{".":{},"f:password":{},"f:username":{}},"f:metadata":{"f:annotations":{".":{},"f:kubectl.kubernetes.io/last-applied-configuration":{}},"f:labels":{".":{},"f:purpose":{}}},"f:type":{}}}]},
					"data":{
						"password":"MWYyZDFlMmU2N2Rm",
						"username":"YWRtaW4="
					},
					"type":"Opaque"
				},
				"dryRun":false,
				"options":{
					"kind":"CreateOptions",
					"apiVersion":"meta.k8s.io/v1",
					"fieldManager":"kubectl-client-side-apply",
					"fieldValidation":"Strict"
				}
			}`),
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			var req admissionv1.AdmissionRequest
			err := json.Unmarshal(c.requestPayload, &req)
			assert.NilError(t, err)
			payload, err := newAdmissionRequestPayload(AdmissionRequest{AdmissionRequest: req})
			assert.NilError(t, err)
			if payload.Object.Object != nil {
				data, err := datautils.ToMap(payload.Object.Object["data"])
				assert.NilError(t, err)
				for _, v := range data {
					assert.Assert(t, v == "**REDACTED**")
				}
				metadata, err := datautils.ToMap(payload.Object.Object["metadata"])
				assert.NilError(t, err)
				annotations, err := datautils.ToMap(metadata["annotations"])
				assert.NilError(t, err)
				for _, v := range annotations {
					assert.Assert(t, v == "**REDACTED**")
				}
			}
			if payload.OldObject.Object != nil {
				data, err := datautils.ToMap(payload.OldObject.Object["data"])
				assert.NilError(t, err)
				for _, v := range data {
					assert.Assert(t, v == "**REDACTED**")
				}
				metadata, err := datautils.ToMap(payload.OldObject.Object["metadata"])
				assert.NilError(t, err)
				annotations, err := datautils.ToMap(metadata["annotations"])
				assert.NilError(t, err)
				for _, v := range annotations {
					assert.Assert(t, v == "**REDACTED**")
				}
			}
		})
	}
}

type testLogEntry struct {
	level  int
	msg    string
	values map[string]any
}

type testSink struct {
	maxLevel int
	entries  *[]testLogEntry
	values   map[string]any
}

func newTestLogger(maxLevel int) (logr.Logger, *[]testLogEntry) {
	var entries []testLogEntry
	sink := &testSink{
		maxLevel: maxLevel,
		entries:  &entries,
		values:   make(map[string]any),
	}
	return logr.New(sink), &entries
}

func (s *testSink) Init(info logr.RuntimeInfo) {}

func (s *testSink) Enabled(level int) bool {
	return level <= s.maxLevel
}

func (s *testSink) Info(level int, msg string, keysAndValues ...any) {
	entryValues := make(map[string]any, len(s.values)+len(keysAndValues)/2)
	for k, v := range s.values {
		entryValues[k] = v
	}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			if key, ok := keysAndValues[i].(string); ok {
				entryValues[key] = keysAndValues[i+1]
			}
		}
	}
	*s.entries = append(*s.entries, testLogEntry{
		level:  level,
		msg:    msg,
		values: entryValues,
	})
}

func (s *testSink) Error(err error, msg string, keysAndValues ...any) {
	s.Info(0, msg, keysAndValues...)
}

func (s *testSink) WithValues(keysAndValues ...any) logr.LogSink {
	newValues := make(map[string]any, len(s.values)+len(keysAndValues)/2)
	for k, v := range s.values {
		newValues[k] = v
	}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			if key, ok := keysAndValues[i].(string); ok {
				newValues[key] = keysAndValues[i+1]
			}
		}
	}
	return &testSink{
		maxLevel: s.maxLevel,
		entries:  s.entries,
		values:   newValues,
	}
}

func (s *testSink) WithName(name string) logr.LogSink {
	return s
}

func TestWithDump(t *testing.T) {
	dummyHandler := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		return AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	podJSON := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"default"}}`)
	podReq := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid-1"),
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Name:      "nginx",
			Namespace: "default",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: podJSON},
		},
	}

	t.Run("dumpPayload enabled at default verbosity (v=2)", func(t *testing.T) {
		logger, entries := newTestLogger(2)
		handler := dummyHandler.WithDump(true)
		resp := handler(context.Background(), logger, podReq, time.Now())
		assert.Assert(t, resp.Allowed)

		var found bool
		for _, e := range *entries {
			if e.msg == "admission request dump" {
				found = true
				assert.Equal(t, 0, e.level)
				reqVal, ok := e.values["admission.request"].(*admissionRequestPayload)
				assert.Assert(t, ok)
				assert.Equal(t, string(reqVal.UID), "test-uid-1")
				assert.Equal(t, reqVal.Name, "nginx")

				respVal, ok := e.values["admission.response"].(AdmissionResponse)
				assert.Assert(t, ok)
				assert.Assert(t, respVal.Allowed)
			}
		}
		assert.Assert(t, found, "expected 'admission request dump' log entry at default verbosity (v=2)")
	})

	t.Run("dumpPayload disabled at default verbosity (v=2)", func(t *testing.T) {
		logger, entries := newTestLogger(2)
		handler := dummyHandler.WithDump(false)
		resp := handler(context.Background(), logger, podReq, time.Now())
		assert.Assert(t, resp.Allowed)

		for _, e := range *entries {
			assert.Assert(t, e.msg != "admission request dump", "dump log should not be emitted when dumpPayload is disabled")
		}
	})
}
