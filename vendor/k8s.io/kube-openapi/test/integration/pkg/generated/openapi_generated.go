// +build !ignore_autogenerated

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

// Code generated by openapi-gen. DO NOT EDIT.

// This file was autogenerated by openapi-gen. Do not edit it manually!

package generated

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"k8s.io/kube-openapi/test/integration/testdata/dummytype.Bar":           schema_test_integration_testdata_dummytype_Bar(ref),
		"k8s.io/kube-openapi/test/integration/testdata/dummytype.Baz":           schema_test_integration_testdata_dummytype_Baz(ref),
		"k8s.io/kube-openapi/test/integration/testdata/dummytype.Foo":           schema_test_integration_testdata_dummytype_Foo(ref),
		"k8s.io/kube-openapi/test/integration/testdata/dummytype.Waldo":         schema_test_integration_testdata_dummytype_Waldo(ref),
		"k8s.io/kube-openapi/test/integration/testdata/listtype.AtomicList":     schema_test_integration_testdata_listtype_AtomicList(ref),
		"k8s.io/kube-openapi/test/integration/testdata/listtype.Item":           schema_test_integration_testdata_listtype_Item(ref),
		"k8s.io/kube-openapi/test/integration/testdata/listtype.MapList":        schema_test_integration_testdata_listtype_MapList(ref),
		"k8s.io/kube-openapi/test/integration/testdata/listtype.SetList":        schema_test_integration_testdata_listtype_SetList(ref),
		"k8s.io/kube-openapi/test/integration/testdata/listtype.UntypedList":    schema_test_integration_testdata_listtype_UntypedList(ref),
		"k8s.io/kube-openapi/test/integration/testdata/uniontype.InlinedUnion":  schema_test_integration_testdata_uniontype_InlinedUnion(ref),
		"k8s.io/kube-openapi/test/integration/testdata/uniontype.TopLevelUnion": schema_test_integration_testdata_uniontype_TopLevelUnion(ref),
		"k8s.io/kube-openapi/test/integration/testdata/uniontype.Union":         schema_test_integration_testdata_uniontype_Union(ref),
		"k8s.io/kube-openapi/test/integration/testdata/uniontype.Union2":        schema_test_integration_testdata_uniontype_Union2(ref),
	}
}

func schema_test_integration_testdata_dummytype_Bar(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"ViolationBehind": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"boolean"},
							Format: "",
						},
					},
					"Violation": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"boolean"},
							Format: "",
						},
					},
				},
				Required: []string{"ViolationBehind", "Violation"},
			},
		},
	}
}

func schema_test_integration_testdata_dummytype_Baz(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Violation": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"boolean"},
							Format: "",
						},
					},
					"ViolationBehind": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"boolean"},
							Format: "",
						},
					},
				},
				Required: []string{"Violation", "ViolationBehind"},
			},
		},
	}
}

func schema_test_integration_testdata_dummytype_Foo(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Second": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"First": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
				Required: []string{"Second", "First"},
			},
		},
	}
}

func schema_test_integration_testdata_dummytype_Waldo(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"First": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"Second": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
				},
				Required: []string{"First", "Second"},
			},
		},
	}
}

func schema_test_integration_testdata_listtype_AtomicList(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Field": {
						VendorExtensible: spec.VendorExtensible{
							Extensions: spec.Extensions{
								"x-kubernetes-list-type": "atomic",
							},
						},
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
				},
				Required: []string{"Field"},
			},
		},
	}
}

func schema_test_integration_testdata_listtype_Item(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Protocol": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"Port": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"a": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"b": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"c": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
				Required: []string{"Protocol", "Port"},
			},
		},
	}
}

func schema_test_integration_testdata_listtype_MapList(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Field": {
						VendorExtensible: spec.VendorExtensible{
							Extensions: spec.Extensions{
								"x-kubernetes-list-map-keys": "port",
								"x-kubernetes-list-type":     "map",
							},
						},
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("k8s.io/kube-openapi/test/integration/testdata/listtype.Item"),
									},
								},
							},
						},
					},
				},
				Required: []string{"Field"},
			},
		},
		Dependencies: []string{
			"k8s.io/kube-openapi/test/integration/testdata/listtype.Item"},
	}
}

func schema_test_integration_testdata_listtype_SetList(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Field": {
						VendorExtensible: spec.VendorExtensible{
							Extensions: spec.Extensions{
								"x-kubernetes-list-type": "set",
							},
						},
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
				},
				Required: []string{"Field"},
			},
		},
	}
}

func schema_test_integration_testdata_listtype_UntypedList(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"Field": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
				},
				Required: []string{"Field"},
			},
		},
	}
}

func schema_test_integration_testdata_uniontype_InlinedUnion(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"name": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"field1": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"field2": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"unionType": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"fieldA": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"fieldB": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"type": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"alpha": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"beta": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
				Required: []string{"name", "type"},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{
					"x-kubernetes-unions": []interface{}{
						map[string]interface{}{
							"discriminator": "unionType",
							"fields-to-discriminateBy": map[string]interface{}{
								"fieldA": "FieldA",
								"fieldB": "FieldB",
							},
						},
						map[string]interface{}{
							"discriminator": "type",
							"fields-to-discriminateBy": map[string]interface{}{
								"alpha": "Alpha",
								"beta":  "Beta",
							},
						},
						map[string]interface{}{
							"fields-to-discriminateBy": map[string]interface{}{
								"field1": "Field1",
								"field2": "Field2",
							},
						},
					},
				},
			},
		},
	}
}

func schema_test_integration_testdata_uniontype_TopLevelUnion(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"name": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"unionType": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"fieldA": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"fieldB": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
				Required: []string{"name"},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{
					"x-kubernetes-unions": []interface{}{
						map[string]interface{}{
							"discriminator": "unionType",
							"fields-to-discriminateBy": map[string]interface{}{
								"fieldA": "FieldA",
								"fieldB": "FieldB",
							},
						},
					},
				},
			},
		},
	}
}

func schema_test_integration_testdata_uniontype_Union(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"unionType": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"fieldA": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"fieldB": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{
					"x-kubernetes-unions": []interface{}{
						map[string]interface{}{
							"discriminator": "unionType",
							"fields-to-discriminateBy": map[string]interface{}{
								"fieldA": "FieldA",
								"fieldB": "FieldB",
							},
						},
					},
				},
			},
		},
	}
}

func schema_test_integration_testdata_uniontype_Union2(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"type": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"alpha": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
					"beta": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"integer"},
							Format: "int32",
						},
					},
				},
				Required: []string{"type"},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{
					"x-kubernetes-unions": []interface{}{
						map[string]interface{}{
							"discriminator": "type",
							"fields-to-discriminateBy": map[string]interface{}{
								"alpha": "Alpha",
								"beta":  "Beta",
							},
						},
					},
				},
			},
		},
	}
}
