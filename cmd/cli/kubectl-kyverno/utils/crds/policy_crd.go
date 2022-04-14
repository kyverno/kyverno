package crds

const PolicyCRD = `
{
	"group": "kyverno.io",
	"names": {
	  "kind": "Policy",
	  "listKind": "PolicyList",
	  "plural": "policies",
	  "shortNames": [
		"pol"
	  ],
	  "singular": "policy"
	},
	"scope": "Namespaced",
	"versions": [
	  {
		"additionalPrinterColumns": [
		  {
			"jsonPath": ".spec.background",
			"name": "Background",
			"type": "string"
		  },
		  {
			"jsonPath": ".spec.validationFailureAction",
			"name": "Action",
			"type": "string"
		  }
		],
		"name": "v1",
		"schema": {
		  "openAPIV3Schema": {
			"description": "Policy declares validation, mutation, and generation behaviors for matching resources. See: https://kyverno.io/docs/writing-policies/ for more information.",
			"properties": {
			  "apiVersion": {
				"description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
				"type": "string"
			  },
			  "kind": {
				"description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds",
				"type": "string"
			  },
			  "metadata": {
				"type": "object"
			  },
			  "spec": {
				"description": "Spec defines policy behaviors and contains one or rules.",
				"properties": {
				  "background": {
					"description": "Background controls if rules are applied to existing resources during a background scan. Optional. Default value is \"true\". The value must be set to \"false\" if the policy rule uses variables that are only available in the admission review request (e.g. user name).",
					"type": "boolean"
				  },
				  "rules": {
					"description": "Rules is a list of Rule instances. A Policy contains multiple rules and each rule can validate, mutate, or generate resources.",
					"items": {
					  "schema": {
						"description": "Rule defines a validation, mutation, or generation control for matching resources. Each rules contains a match declaration to select resources, and an optional exclude declaration to specify which resources to exclude.",
						"properties": {
						  "context": {
							"description": "Context defines variables and data sources that can be used during rule execution.",
							"items": {
							  "schema": {
								"description": "ContextEntry adds variables and data sources to a rule Context. Either a ConfigMap reference or a APILookup must be provided.",
								"properties": {
								  "apiCall": {
									"description": "APICall defines an HTTP request to the Kubernetes API server. The JSON data retrieved is stored in the context.",
									"properties": {
									  "jmesPath": {
										"description": "JMESPath is an optional JSON Match Expression that can be used to transform the JSON response returned from the API server. For example a JMESPath of \"items | length(@)\" applied to the API server response to the URLPath \"/apis/apps/v1/deployments\" will return the total count of deployments across all namespaces.",
										"type": "string"
									  },
									  "urlPath": {
										"description": "URLPath is the URL path to be used in the HTTP GET request to the Kubernetes API server (e.g. \"/api/v1/namespaces\" or  \"/apis/apps/v1/deployments\"). The format required is the same format used by the 'kubectl get --raw' command.",
										"type": "string"
									  }
									},
									"required": [
									  "urlPath"
									],
									"type": "object"
								  },
								  "configMap": {
									"description": "ConfigMap is the ConfigMap reference.",
									"properties": {
									  "name": {
										"description": "Name is the ConfigMap name.",
										"type": "string"
									  },
									  "namespace": {
										"description": "Namespace is the ConfigMap namespace.",
										"type": "string"
									  }
									},
									"required": [
									  "name"
									],
									"type": "object"
								  },
								  "name": {
									"description": "Name is the variable name.",
									"type": "string"
								  }
								},
								"type": "object"
							  }
							},
							"type": "array"
						  },
						  "exclude": {
							"description": "ExcludeResources defines when this policy rule should not be applied. The exclude criteria can include resource information (e.g. kind, name, namespace, labels) and admission review request information like the name or role.",
							"properties": {
							  "clusterRoles": {
								"description": "ClusterRoles is the list of cluster-wide role names for the user.",
								"items": {
								  "schema": {
									"type": "string"
								  }
								},
								"type": "array"
							  },
							  "resources": {
								"description": "ResourceDescription contains information about the resource being created or modified.",
								"properties": {
								  "annotations": {
									"description": "Annotations is a  map of annotations (key-value pairs of type string). Annotation keys and values support the wildcard characters \"*\" (matches zero or many characters) and \"?\" (matches at least one character).",
									"type": "object"
								  },
								  "kinds": {
									"description": "Kinds is a list of resource kinds.",
									"items": {
									  "schema": {
										"type": "string"
									  }
									},
									"type": "array"
								  },
								  "name": {
									"description": "Name is the name of the resource. The name supports wildcard characters \"*\" (matches zero or many characters) and \"?\" (at least one character).",
									"type": "string"
								  },
								  "namespaceSelector": {
									"description": "NamespaceSelector is a label selector for the resource namespace. Label keys and values in 'matchLabels' support the wildcard characters '*' (matches zero or many characters) and '?' (matches one character).Wildcards allows writing label selectors like [\"storage.k8s.io/*\": \"*\"]. Note that using [\"*\" : \"*\"] matches any key and value but does not match an empty label set.",
									"properties": {
									  "matchExpressions": {
										"description": "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
										"items": {
										  "schema": {
											"description": "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
											"properties": {
											  "key": {
												"description": "key is the label key that the selector applies to.",
												"type": "string"
											  },
											  "operator": {
												"description": "operator represents a key's relationship to a set of values. Valid operators are In, AnyIn, AllIn, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn AnyNotIn, AllNotIn, Exists and DoesNotExist.",
												"type": "string"
											  },
											  "values": {
												"description": "values is an array of string values. If the operator is In, AnyIn, AllIn, NotIn, AnyNotIn or AllNotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
												"items": {
												  "schema": {
													"type": "string"
												  }
												},
												"type": "array"
											  }
											},
											"required": [
											  "key",
											  "operator"
											],
											"type": "object"
										  }
										},
										"type": "array"
									  },
									  "matchLabels": {
										"description": "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
										"type": "object"
									  }
									},
									"type": "object"
								  },
								  "namespaces": {
									"description": "Namespaces is a list of namespaces names. Each name supports wildcard characters \"*\" (matches zero or many characters) and \"?\" (at least one character).",
									"items": {
									  "schema": {
										"type": "string"
									  }
									},
									"type": "array"
								  },
								  "selector": {
									"description": "Selector is a label selector. Label keys and values in 'matchLabels' support the wildcard characters '*' (matches zero or many characters) and '?' (matches one character). Wildcards allows writing label selectors like [\"storage.k8s.io/*\": \"*\"]. Note that using [\"*\" : \"*\"] matches any key and value but does not match an empty label set.",
									"properties": {
									  "matchExpressions": {
										"description": "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
										"items": {
										  "schema": {
											"description": "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
											"properties": {
											  "key": {
												"description": "key is the label key that the selector applies to.",
												"type": "string"
											  },
											  "operator": {
												"description": "operator represents a key's relationship to a set of values. Valid operators are In, AnyIn, AllIn, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn AnyNotIn, AllNotIn, Exists and DoesNotExist.",
												"type": "string"
											  },
											  "values": {
												"description": "values is an array of string values. If the operator is In, AnyIn, AllIn, NotIn, AnyNotIn or AllNotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
												"items": {
												  "schema": {
													"type": "string"
												  }
												},
												"type": "array"
											  }
											},
											"required": [
											  "key",
											  "operator"
											],
											"type": "object"
										  }
										},
										"type": "array"
									  },
									  "matchLabels": {
										"description": "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
										"type": "object"
									  }
									},
									"type": "object"
								  }
								},
								"type": "object"
							  },
							  "roles": {
								"description": "Roles is the list of namespaced role names for the user.",
								"items": {
								  "schema": {
									"type": "string"
								  }
								},
								"type": "array"
							  },
							  "subjects": {
								"description": "Subjects is the list of subject names like users, user groups, and service accounts.",
								"items": {
								  "schema": {
									"description": "Subject contains a reference to the object or user identities a role binding applies to.  This can either hold a direct API object reference, or a value for non-objects such as user and group names.",
									"properties": {
									  "apiGroup": {
										"description": "APIGroup holds the API group of the referenced subject. Defaults to \"\" for ServiceAccount subjects. Defaults to \"rbac.authorization.k8s.io\" for User and Group subjects.",
										"type": "string"
									  },
									  "kind": {
										"description": "Kind of object being referenced. Values defined by this API group are \"User\", \"Group\", and \"ServiceAccount\". If the Authorizer does not recognized the kind value, the Authorizer should report an error.",
										"type": "string"
									  },
									  "name": {
										"description": "Name of the object being referenced.",
										"type": "string"
									  },
									  "namespace": {
										"description": "Namespace of the referenced object.  If the object kind is non-namespace, such as \"User\" or \"Group\", and this value is not empty the Authorizer should report an error.",
										"type": "string"
									  }
									},
									"required": [
									  "kind",
									  "name"
									],
									"type": "object"
								  }
								},
								"type": "array"
							  }
							},
							"type": "object"
						  },
						  "generate": {
							"description": "Generation is used to create new resources.",
							"properties": {
							  "apiVersion": {
								"description": "APIVersion specifies resource apiVersion.",
								"type": "string"
							  },
							  "clone": {
								"description": "Clone specifies the source resource used to populate each generated resource. At most one of Data or Clone can be specified. If neither are provided, the generated resource will be created with default data only.",
								"properties": {
								  "name": {
									"description": "Name specifies name of the resource.",
									"type": "string"
								  },
								  "namespace": {
									"description": "Namespace specifies source resource namespace.",
									"type": "string"
								  }
								},
								"type": "object"
							  },
							  "data": {
								"description": "Data provides the resource declaration used to populate each generated resource. At most one of Data or Clone must be specified. If neither are provided, the generated resource will be created with default data only.",
								"x-kubernetes-preserve-unknown-fields": true
							  },
							  "kind": {
								"description": "Kind specifies resource kind.",
								"type": "string"
							  },
							  "name": {
								"description": "Name specifies the resource name.",
								"type": "string"
							  },
							  "namespace": {
								"description": "Namespace specifies resource namespace.",
								"type": "string"
							  },
							  "synchronize": {
								"description": "Synchronize controls if generated resources should be kept in-sync with their source resource. If Synchronize is set to \"true\" changes to generated resources will be overwritten with resource data from Data or the resource specified in the Clone declaration. Optional. Defaults to \"false\" if not specified.",
								"type": "boolean"
							  }
							},
							"type": "object"
						  },
						  "match": {
							"description": "MatchResources defines when this policy rule should be applied. The match criteria can include resource information (e.g. kind, name, namespace, labels) and admission review request information like the user name or role. At least one kind is required.",
							"properties": {
							  "clusterRoles": {
								"description": "ClusterRoles is the list of cluster-wide role names for the user.",
								"items": {
								  "schema": {
									"type": "string"
								  }
								},
								"type": "array"
							  },
							  "resources": {
								"description": "ResourceDescription contains information about the resource being created or modified. Requires at least one tag to be specified when under MatchResources.",
								"properties": {
								  "annotations": {
									"description": "Annotations is a  map of annotations (key-value pairs of type string). Annotation keys and values support the wildcard characters \"*\" (matches zero or many characters) and \"?\" (matches at least one character).",
									"type": "object"
								  },
								  "kinds": {
									"description": "Kinds is a list of resource kinds.",
									"items": {
									  "schema": {
										"type": "string"
									  }
									},
									"type": "array"
								  },
								  "name": {
									"description": "Name is the name of the resource. The name supports wildcard characters \"*\" (matches zero or many characters) and \"?\" (at least one character).",
									"type": "string"
								  },
								  "namespaceSelector": {
									"description": "NamespaceSelector is a label selector for the resource namespace. Label keys and values in 'matchLabels' support the wildcard characters '*' (matches zero or many characters) and '?' (matches one character).Wildcards allows writing label selectors like [\"storage.k8s.io/*\": \"*\"]. Note that using [\"*\" : \"*\"] matches any key and value but does not match an empty label set.",
									"properties": {
									  "matchExpressions": {
										"description": "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
										"items": {
										  "schema": {
											"description": "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
											"properties": {
											  "key": {
												"description": "key is the label key that the selector applies to.",
												"type": "string"
											  },
											  "operator": {
												"description": "operator represents a key's relationship to a set of values. Valid operators are In, AnyIn, AllIn, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn AnyNotIn, AllNotIn, Exists and DoesNotExist.",
												"type": "string"
											  },
											  "values": {
												"description": "values is an array of string values. If the operator is In, AnyIn, AllIn, NotIn, AnyNotIn or AllNotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
												"items": {
												  "schema": {
													"type": "string"
												  }
												},
												"type": "array"
											  }
											},
											"required": [
											  "key",
											  "operator"
											],
											"type": "object"
										  }
										},
										"type": "array"
									  },
									  "matchLabels": {
										"description": "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
										"type": "object"
									  }
									},
									"type": "object"
								  },
								  "namespaces": {
									"description": "Namespaces is a list of namespaces names. Each name supports wildcard characters \"*\" (matches zero or many characters) and \"?\" (at least one character).",
									"items": {
									  "schema": {
										"type": "string"
									  }
									},
									"type": "array"
								  },
								  "selector": {
									"description": "Selector is a label selector. Label keys and values in 'matchLabels' support the wildcard characters '*' (matches zero or many characters) and '?' (matches one character). Wildcards allows writing label selectors like [\"storage.k8s.io/*\": \"*\"]. Note that using [\"*\" : \"*\"] matches any key and value but does not match an empty label set.",
									"properties": {
									  "matchExpressions": {
										"description": "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
										"items": {
										  "schema": {
											"description": "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
											"properties": {
											  "key": {
												"description": "key is the label key that the selector applies to.",
												"type": "string"
											  },
											  "operator": {
												"description": "operator represents a key's relationship to a set of values. Valid operators are In, AnyIn, AllIn, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn AnyNotIn, AllNotIn, Exists and DoesNotExist.",
												"type": "string"
											  },
											  "values": {
												"description": "values is an array of string values. If the operator is In, AnyIn, AllIn, NotIn, AnyNotIn or AllNotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
												"items": {
												  "schema": {
													"type": "string"
												  }
												},
												"type": "array"
											  }
											},
											"required": [
											  "key",
											  "operator"
											],
											"type": "object"
										  }
										},
										"type": "array"
									  },
									  "matchLabels": {
										"description": "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
										"type": "object"
									  }
									},
									"type": "object"
								  }
								},
								"type": "object"
							  },
							  "roles": {
								"description": "Roles is the list of namespaced role names for the user.",
								"items": {
								  "schema": {
									"type": "string"
								  }
								},
								"type": "array"
							  },
							  "subjects": {
								"description": "Subjects is the list of subject names like users, user groups, and service accounts.",
								"items": {
								  "schema": {
									"description": "Subject contains a reference to the object or user identities a role binding applies to.  This can either hold a direct API object reference, or a value for non-objects such as user and group names.",
									"properties": {
									  "apiGroup": {
										"description": "APIGroup holds the API group of the referenced subject. Defaults to \"\" for ServiceAccount subjects. Defaults to \"rbac.authorization.k8s.io\" for User and Group subjects.",
										"type": "string"
									  },
									  "kind": {
										"description": "Kind of object being referenced. Values defined by this API group are \"User\", \"Group\", and \"ServiceAccount\". If the Authorizer does not recognized the kind value, the Authorizer should report an error.",
										"type": "string"
									  },
									  "name": {
										"description": "Name of the object being referenced.",
										"type": "string"
									  },
									  "namespace": {
										"description": "Namespace of the referenced object.  If the object kind is non-namespace, such as \"User\" or \"Group\", and this value is not empty the Authorizer should report an error.",
										"type": "string"
									  }
									},
									"required": [
									  "kind",
									  "name"
									],
									"type": "object"
								  }
								},
								"type": "array"
							  }
							},
							"type": "object"
						  },
						  "mutate": {
							"description": "Mutation is used to modify matching resources.",
							"properties": {
							  "overlay": {
								"description": "Overlay specifies an overlay pattern to modify resources. DEPRECATED. Use PatchStrategicMerge instead. Scheduled for removal in release 1.5+.",
								"x-kubernetes-preserve-unknown-fields": true
							  },
							  "patchStrategicMerge": {
								"description": "PatchStrategicMerge is a strategic merge patch used to modify resources. See https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/ and https://kubectl.docs.kubernetes.io/references/kustomize/patchesstrategicmerge/.",
								"x-kubernetes-preserve-unknown-fields": true
							  },
							  "patches": {
								"description": "Patches specifies a RFC 6902 JSON Patch to modify resources. DEPRECATED. Use PatchesJSON6902 instead. Scheduled for removal in release 1.5+.",
								"items": {
								  "schema": {
									"description": "Patch is a RFC 6902 JSON Patch. See: https://tools.ietf.org/html/rfc6902",
									"properties": {
									  "op": {
										"description": "Operation specifies operations supported by JSON Patch. i.e:- add, replace and delete.",
										"type": "string"
									  },
									  "path": {
										"description": "Path specifies path of the resource.",
										"type": "string"
									  },
									  "value": {
										"description": "Value specifies the value to be applied.",
										"x-kubernetes-preserve-unknown-fields": true
									  }
									},
									"type": "object"
								  }
								},
								"nullable": true,
								"type": "array",
								"x-kubernetes-preserve-unknown-fields": true
							  },
							  "patchesJson6902": {
								"description": "PatchesJSON6902 is a list of RFC 6902 JSON Patch declarations used to modify resources. See https://tools.ietf.org/html/rfc6902 and https://kubectl.docs.kubernetes.io/references/kustomize/patchesjson6902/.",
								"type": "string"
							  }
							},
							"type": "object"
						  },
						  "name": {
							"description": "Name is a label to identify the rule, It must be unique within the policy.",
							"maxLength": 63,
							"type": "string"
						  },
						  "preconditions": {
							"description": "AnyAllConditions enable variable-based conditional rule execution. This is useful for finer control of when an rule is applied. A condition can reference object data using JMESPath notation. This too can be made to happen in a logical-manner where in some situation all the conditions need to pass and in some other situation, atleast one condition is enough to pass. For the sake of backwards compatibility, it can be populated with []kyverno.Condition.",
							"x-kubernetes-preserve-unknown-fields": true
						  },
						  "validate": {
							"description": "Validation is used to validate matching resources.",
							"properties": {
							  "anyPattern": {
								"description": "AnyPattern specifies list of validation patterns. At least one of the patterns must be satisfied for the validation rule to succeed.",
								"x-kubernetes-preserve-unknown-fields": true
							  },
							  "deny": {
								"description": "Deny defines conditions to fail the validation rule.",
								"properties": {
								  "conditions": {
									"description": "specifies the set of conditions to deny in a logical manner For the sake of backwards compatibility, it can be populated with []kyverno.Condition.",
									"x-kubernetes-preserve-unknown-fields": true
								  }
								},
								"type": "object"
							  },
							  "message": {
								"description": "Message specifies a custom message to be displayed on failure.",
								"type": "string"
							  },
							  "pattern": {
								"description": "Pattern specifies an overlay-style pattern used to check resources.",
								"x-kubernetes-preserve-unknown-fields": true
							  }
							},
							"type": "object"
						  }
						},
						"type": "object"
					  }
					},
					"type": "array"
				  },
				  "validationFailureAction": {
					"description": "ValidationFailureAction controls if a validation policy rule failure should disallow the admission review request (enforce), or allow (audit) the admission review request and report an error in a policy report. Optional. The default value is \"audit\".",
					"type": "string"
				  }
				},
				"type": "object"
			  },
			  "status": {
				"description": "Status contains policy runtime information.",
				"properties": {
				  "averageExecutionTime": {
					"description": "AvgExecutionTime is the average time taken to process the policy rules on a resource.",
					"type": "string"
				  },
				  "resourcesBlockedCount": {
					"description": "ResourcesBlockedCount is the total count of admission review requests that were blocked by this policy.",
					"type": "integer"
				  },
				  "resourcesGeneratedCount": {
					"description": "ResourcesGeneratedCount is the total count of resources that were generated by this policy.",
					"type": "integer"
				  },
				  "resourcesMutatedCount": {
					"description": "ResourcesMutatedCount is the total count of resources that were mutated by this policy.",
					"type": "integer"
				  },
				  "ruleStatus": {
					"description": "Rules provides per rule statistics",
					"items": {
					  "schema": {
						"description": "RuleStats provides statistics for an individual rule within a policy.",
						"properties": {
						  "appliedCount": {
							"description": "AppliedCount is the total number of times this rule was applied.",
							"type": "integer"
						  },
						  "averageExecutionTime": {
							"description": "ExecutionTime is the average time taken to execute this rule.",
							"type": "string"
						  },
						  "failedCount": {
							"description": "FailedCount is the total count of policy error results for this rule.",
							"type": "integer"
						  },
						  "resourcesBlockedCount": {
							"description": "ResourcesBlockedCount is the total count of admission review requests that were blocked by this rule.",
							"type": "integer"
						  },
						  "resourcesGeneratedCount": {
							"description": "ResourcesGeneratedCount is the total count of resources that were generated by this rule.",
							"type": "integer"
						  },
						  "resourcesMutatedCount": {
							"description": "ResourcesMutatedCount is the total count of resources that were mutated by this rule.",
							"type": "integer"
						  },
						  "ruleName": {
							"description": "Name is the rule name.",
							"type": "string"
						  },
						  "violationCount": {
							"description": "ViolationCount is the total count of policy failure results for this rule.",
							"type": "integer"
						  }
						},
						"required": [
						  "ruleName"
						],
						"type": "object"
					  }
					},
					"type": "array"
				  },
				  "rulesAppliedCount": {
					"description": "RulesAppliedCount is the total number of times this policy was applied.",
					"type": "integer"
				  },
				  "rulesFailedCount": {
					"description": "RulesFailedCount is the total count of policy execution errors for this policy.",
					"type": "integer"
				  },
				  "violationCount": {
					"description": "ViolationCount is the total count of policy failure results for this policy.",
					"type": "integer"
				  }
				},
				"type": "object"
			  }
			},
			"required": [
			  "spec"
			],
			"type": "object"
		  }
		},
		"served": true,
		"storage": true,
		"subresources": {
		  "status": {}
		}
	  }
	]
  }
`
