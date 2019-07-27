package engine

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func TestMeetConditions_NoAnchor(t *testing.T) {
	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)
	var overlay interface{}

	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(nil, overlay)
	assert.Assert(t, res)
}

func TestMeetConditions_invalidConditionalAnchor(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)

	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "(ports)":[  
				 {  
					"name":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, !res)

	overlayRaw = []byte(`
	{  
	   "(subsets)":[  
		  {  
			 "ports":[  
				{  
				   "name":"secure-connection",
				   "port":444,
				   "protocol":"UDP"
				}
			 ]
		  }
	   ]
	}`)

	json.Unmarshal(overlayRaw, &overlay)

	res = meetConditions(resource, overlay)
	assert.Assert(t, !res)
}

func TestMeetConditions_DifferentTypes(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
		   }
		]
	 }`)

	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "ports":[  
				 {  
					"(name)":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)
	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	// anchor exist
	res := meetConditions(resource, overlay)
	assert.Assert(t, !res)
}

func TestMeetConditions_anchosInSameObject(t *testing.T) {
	resourceRaw := []byte(`
	{  
	   "apiVersion":"v1",
	   "kind":"Endpoints",
	   "metadata":{  
		  "name":"test-endpoint",
		  "labels":{  
			 "label":"test"
		  }
	   },
	   "subsets":[  
		  {  
			 "addresses":[  
				{  
				   "ip":"192.168.10.171"
				}
			 ],
			 "ports":[  
				{  
				   "name":"secure-connection",
				   "port":443,
				   "protocol":"TCP"
				}
			 ]
		  }
	   ]
	}`)

	overlayRaw := []byte(`
	{  
	   "subsets":[  
		  {  
			 "ports":[  
				{  
				   "(name)":"secure-connection",
				   "(port)":444,
				   "protocol":"UDP"
				}
			 ]
		  }
	   ]
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	// no anchor
	res := meetConditions(resource, overlay)
	assert.Assert(t, !res)
}

func TestMeetConditions_anchorOnPeer(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)

	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			"addresses":[  
				{  
				   "(ip)":"192.168.10.171"
				}
			 ],
			  "ports":[  
				 {  
					"(name)":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, res)
}

func TestMeetConditions_anchorsOnMetaAndSpec(t *testing.T) {
	overlayRaw := []byte(`{
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"(app)": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"imagePullPolicy": "IfNotPresent",
							"ports": [
								{
									"containerPort": 8080
								}
							]
						}
					]
				}
			}
		}
	}`)
	resourceRaw := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "nginx-deployment",
			"labels": {
				"app": "nginx"
			}
		},
		"spec": {
			"replicas": 1,
			"selector": {
				"matchLabels": {
					"app": "nginx"
				}
			},
			"template": {
				"metadata": {
					"labels": {
						"app": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"name": "nginx",
							"image": "nginx:latest",
							"ports": [
								{
									"containerPort": 80
								}
							]
						},
						{
							"name": "ghost",
							"image": "ghost:latest"
						}
					]
				}
			}
		}
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, res)
}

var resourceRawAnchorOnPeers = []byte(`{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
	   "name": "psp-demo-unprivileged",
	   "labels": {
		  "app.type": "prod"
	   }
	},
	"spec": {
	   "replicas": 1,
	   "selector": {
		  "matchLabels": {
			 "app": "psp"
		  }
	   },
	   "template": {
		  "metadata": {
			 "labels": {
				"app": "psp"
			 }
		  },
		  "spec": {
			 "securityContext": {
				"runAsNonRoot": true
			 },
			 "containers": [
				{
				   "name": "sec-ctx-unprivileged",
				   "image": "nginxinc/nginx-unprivileged",
				   "securityContext": {
					  "runAsNonRoot": true,
					  "allowPrivilegeEscalation": false
				   },
				   "env": [
					  {
						 "name": "ENV_KEY",
						 "value": "ENV_VALUE"
					  }
				   ]
				}
			 ]
		  }
	   }
	}
 }`)

func TestMeetConditions_anchorsOnPeer_single(t *testing.T) {
	overlayRaw := []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "allowPrivilegeEscalation": false
					   },
					   "env": [
						  {
							 "name": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRawAnchorOnPeers, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, res)
}

func TestMeetConditions_anchorsOnPeer_two(t *testing.T) {
	overlayRaw := []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "(runAsNonRoot)": true,
						  "allowPrivilegeEscalation": false
					   },
					   "env": [
						  {
							 "name": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRawAnchorOnPeers, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, res)

	overlayRaw = []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "allowPrivilegeEscalation": false
					   },
					   "env": [
						  {
							 "(name)": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	json.Unmarshal(overlayRaw, &overlay)

	res = meetConditions(resource, overlay)
	assert.Assert(t, res)

	overlayRaw = []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "image": "*/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "(allowPrivilegeEscalation)": false
					   },
					   "env": [
						  {
							 "(name)": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	json.Unmarshal(overlayRaw, &overlay)

	res = meetConditions(resource, overlay)
	assert.Assert(t, res)
}

func TestMeetConditions_anchorsOnPeer_multiple(t *testing.T) {
	overlayRaw := []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "(runAsNonRoot)": true,
						  "allowPrivilegeEscalation": false
					   },
					   "env": [
						  {
							 "(name)": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRawAnchorOnPeers, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := meetConditions(resource, overlay)
	assert.Assert(t, res)

	overlayRaw = []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "(allowPrivilegeEscalation)": false
					   },
					   "env": [
						  {
							 "(name)": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	json.Unmarshal(overlayRaw, &overlay)

	res = meetConditions(resource, overlay)
	assert.Assert(t, res)

	overlayRaw = []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "(allowPrivilegeEscalation)": false
					   },
					   "env": [
						  {
							 "(name)": "ENV_KEY",
							 "(value)": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	json.Unmarshal(overlayRaw, &overlay)

	res = meetConditions(resource, overlay)
	assert.Assert(t, res)

}
