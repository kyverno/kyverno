package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	log "github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"gotest.tools/assert"
)

func TestValidate_failure_action_overrides(t *testing.T) {
	testcases := []struct {
		rawPolicy                  []byte
		rawResource                []byte
		blocked                    bool
		messages                   map[string]string
		rawResourceNamespaceLabels map[string]string
	}{
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
		 	`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							"name": "check-label-app",
							"match": {
							   "resources": {
								  "kinds": [
									 "Pod"
								  ]
							   }
							},
							"validate": {
							   "message": "The label 'app' is required.",
							   "pattern": {
								   "metadata": {
									   "labels": {
										   "app": "?*"
									   }
								   }
							   }
						   }
						 }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": ""
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "audit",
									"namespaces": [
										"dev"
									],
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "audit",
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "prod"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "prod",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "audit",
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "audit",
									"namespaces": [
									  "dev"
									],
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides":
							[
								{
									"action": "audit",
									"namespaces": [
									  "dev"
									],
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "prod"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "prod",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
									  "dev"
									],
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "prod"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		}, {
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides":
							[
								{
									"action": "enforce",
									"namespaces": [
									  "dev"
									],
									"namespaceSelector": {
										"matchExpressions": [{
										  "key" : "kubernetes.io/metadata.name",
                      "operator": "In",
                      "values": [
										 	  "dev"
											]
										}]
									}
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "audit",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "audit",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "audit",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
		 	`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							"name": "check-label-app",
							"match": {
							   "resources": {
								  "kinds": [
									 "Pod"
								  ]
							   }
							},
							"validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									[
										{
											"action": "enforce",
											"namespaces": [
												"default"
											]
										},
										{
											"action": "audit",
											"namespaces": [
												"test"
											]
										}
									],
							   "message": "The label 'app' is required.",
							   "pattern": {
								   "metadata": {
									   "labels": {
										   "app": "?*"
									   }
								   }
							   }
						   }
						 }
					   ]
					}
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": ""
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									[
										{
											"action": "audit",
											"namespaces": [
												"dev"
											],
											"namespaceSelector": {
												"matchExpressions": [{
													"key" : "kubernetes.io/metadata.name",
													"operator": "In",
													"values": [
														"prod"
													]
												}]
											}
										}
									],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									 [
										 {
											 "action": "audit",
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
												   "operator": "In",
												   "values": [
														"prod"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "prod"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "prod",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									 [
										 {
											 "action": "audit",
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
												   "operator": "In",
												   "values": [
														"prod"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									 [
										 {
											 "action": "audit",
											 "namespaces": [
											   "dev"
											 ],
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
												   "operator": "In",
												   "values": [
														"prod"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "enforce",
								"failureActionOverrides":
									 [
										 {
											 "action": "audit",
											 "namespaces": [
											   "dev"
											 ],
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
												   "operator": "In",
												   "values": [
														"prod"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "prod"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "prod",
			},
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "audit",
								"failureActionOverrides":
									 [
										 {
											 "action": "enforce",
											 "namespaces": [
											   "dev"
											 ],
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
													"operator": "In",
													"values": [
														"prod"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		}, {
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"failureAction": "audit",
								"failureActionOverrides":
									 [
										 {
											 "action": "enforce",
											 "namespaces": [
											   "dev"
											 ],
											 "namespaceSelector": {
												 "matchExpressions": [{
												   "key" : "kubernetes.io/metadata.name",
												   "operator": "In",
												   "values": [
														"dev"
													 ]
												 }]
											 }
										 }
									 ],
								"message": "The label 'app' is required.",
								"pattern": {
									"metadata": {
										"labels": {
											"app": "?*"
										}
									}
								}
							}
						  }
					   ]
					}
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
						 "namespace": "dev"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			rawResourceNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "dev",
			},
		},
	}
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	rclient := registryclient.NewOrDie()
	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
	)
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal(tc.rawPolicy, &policy)
			assert.NilError(t, err)
			resourceUnstructured, err := kubeutils.BytesToUnstructured(tc.rawResource)
			assert.NilError(t, err)

			ctx, err := engine.NewPolicyContext(
				jp,
				*resourceUnstructured,
				kyvernov1.Create,
				nil,
				cfg,
			)
			assert.NilError(t, err)

			ctx = ctx.WithPolicy(&policy).WithNamespaceLabels(tc.rawResourceNamespaceLabels)
			er := eng.Validate(
				context.TODO(),
				ctx,
			)
			if tc.blocked && tc.messages != nil {
				for _, r := range er.PolicyResponse.Rules {
					msg := tc.messages[r.Name()]
					assert.Equal(t, r.Message(), msg)
				}
			}

			failurePolicy := kyvernov1.Fail
			blocked := webhookutils.BlockRequest([]engineapi.EngineResponse{er}, failurePolicy, log.WithName("WebhookServer"))
			assert.Assert(t, tc.blocked == blocked)
		})
	}
}

func Test_RuleSelector(t *testing.T) {
	var rawPolicy = []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "check-label-app"},
		"spec": {
		   "validationFailureAction": "enforce",
		   "rules": [
			  {
				"name": "check-label-test",
				"match": {"name": "test-*", "resources": {"kinds": ["Pod"]}},
				"validate": {
				   "message": "The label 'app' is required.",
				   "pattern": { "metadata": { "labels": { "app": "?*" } } }
				}
			  },
			  {
				"name": "check-labels",
				"match": {"name": "*", "resources": {"kinds": ["Pod"]}},
				"validate": {
				   "message": "The label 'app' is required.",
				   "pattern": { "metadata": { "labels": { "app": "?*", "test" : "?*" } } }
				}
			  }
		   ]
		}
	 }`)

	var rawResource = []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "test-pod", "namespace": "", "labels": { "app" : "test-pod" }},
		"spec": {"containers": [{"name": "nginx", "image": "nginx:latest"}]}
	}`)

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	assert.Assert(t, resourceUnstructured != nil)

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	ctx, err := engine.NewPolicyContext(
		jp,
		*resourceUnstructured,
		kyvernov1.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)

	ctx = ctx.WithPolicy(&policy)
	rclient := registryclient.NewOrDie()
	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
	)
	resp := eng.Validate(
		context.TODO(),
		ctx,
	)
	assert.Assert(t, resp.PolicyResponse.RulesAppliedCount() == 2)
	assert.Assert(t, resp.PolicyResponse.RulesErrorCount() == 0)

	log := log.WithName("Test_RuleSelector")
	blocked := webhookutils.BlockRequest([]engineapi.EngineResponse{resp}, kyvernov1.Fail, log)
	assert.Assert(t, blocked == true)

	applyOne := kyvernov1.ApplyOne
	policy.Spec.ApplyRules = &applyOne
	resp = eng.Validate(
		context.TODO(),
		ctx,
	)
	assert.Assert(t, resp.PolicyResponse.RulesAppliedCount() == 1)
	assert.Assert(t, resp.PolicyResponse.RulesErrorCount() == 0)

	blocked = webhookutils.BlockRequest([]engineapi.EngineResponse{resp}, kyvernov1.Fail, log)
	assert.Assert(t, blocked == false)
}
