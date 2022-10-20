package webhooks

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
	admissionv1 "k8s.io/api/admission/v1"
)

func Test_RedactPayload(t *testing.T) {
	req := admissionv1.AdmissionRequest{}
	q := []byte(`{
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
	}`)
	err := json.Unmarshal(q, &req)
	assert.NilError(t, err)
	_, err = newAdmissionRequestPayload(&req)
	assert.NilError(t, err)
}
