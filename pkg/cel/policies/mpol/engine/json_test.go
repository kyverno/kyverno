package engine

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func jsonMutationPolicy(name string, expressions ...string) *policiesv1beta1.MutatingPolicy {
	policy := &policiesv1beta1.MutatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
	}
	for _, expression := range expressions {
		policy.Spec.Mutations = append(policy.Spec.Mutations, admissionregistrationv1alpha1.Mutation{
			PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
			JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression},
		})
	}
	return policy
}

func TestJSONEngineDocuments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, expression, expected string
	}{
		{"object", `{"region":"old"}`, `[JSONPatch{op:"replace",path:"/region",value:"new"}]`, `{"region":"new"}`},
		{"array append", `[1,"two",null]`, `[JSONPatch{op:"add",path:"/-",value:{"enabled":true}}]`, `[1,"two",null,{"enabled":true}]`},
		{"string root", `"old"`, `[JSONPatch{op:"replace",path:"",value:"new"}]`, `"new"`},
		{"number root", `7`, `[JSONPatch{op:"add",path:"",value:object + 1}]`, `8`},
		{"boolean root", `false`, `[JSONPatch{op:"replace",path:"",value:!object}]`, `true`},
		{"null root", `null`, `[JSONPatch{op:"replace",path:"",value:{"a":null,"b":[1,"two",false]}}]`, `{"a":null,"b":[1,"two",false]}`},
		{"replace with null", `{"a":1}`, `[JSONPatch{op:"replace",path:"",value:null}]`, `null`},
		{"escaped pointer", `{}`, `[JSONPatch{op:"add",path:"/" + jsonpatch.escapeKey("a/b~c"),value:true}]`, `{"a/b~c":true}`},
		{"remove", `{"a":1,"b":2}`, `[JSONPatch{op:"remove",path:"/a"}]`, `{"b":2}`},
		{"move", `{"a":1}`, `[JSONPatch{op:"move",from:"/a",path:"/b"}]`, `{"b":1}`},
		{"copy root", `{"a":1}`, `[JSONPatch{op:"copy",from:"",path:"/b"}]`, `{"a":1,"b":{"a":1}}`},
		{"move to root", `{"a":[1,2]}`, `[JSONPatch{op:"move",from:"/a",path:""}]`, `[1,2]`},
		{"test root", `null`, `[JSONPatch{op:"test",path:"",value:null},JSONPatch{op:"replace",path:"",value:"ok"}]`, `"ok"`},
		{"empty patch", `{"a":1}`, `[JSONPatch{op:"add",path:"/unused",value:1}].filter(x, false)`, `{"a":1}`},
		{"large int value", `{"id":9007199254740993}`, `[JSONPatch{op:"add",path:"/copy",value:object.id}]`, `{"id":9007199254740993,"copy":9007199254740993}`},
		{"uint value in int64 range", `{}`, `[JSONPatch{op:"add",path:"/u",value:9223372036854775807u}]`, `{"u":9223372036854775807}`},
		{"unchanged decimal", `{"decimal":0.1234567890123456789012345}`, `[JSONPatch{op:"add",path:"/ok",value:true}]`, `{"decimal":0.1234567890123456789012345,"ok":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", tt.expression)}, nil)
			require.NoError(t, err)
			input := json.RawMessage(tt.input)
			response, err := engine.HandleJSON(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, tt.input, string(input))
			// Compact raw comparison also checks integer/decimal lexemes without float64 decoding.
			require.Equal(t, compactJSON(t, json.RawMessage(tt.expected)), compactJSON(t, response.Document))
			require.Equal(t, JSONPolicyApplied, response.Policies[0].Status)
		})
	}
}

func compactJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var document any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&document))
	bytes, err := json.Marshal(document)
	require.NoError(t, err)
	return string(bytes)
}

func TestJSONEngineSequential(t *testing.T) {
	t.Parallel()
	first := jsonMutationPolicy("z-first",
		`[JSONPatch{op:"add",path:"/count",value:1}]`,
		`[JSONPatch{op:"replace",path:"/count",value:variables.count + 1}]`,
		`[JSONPatch{op:"replace",path:"/count",value:variables.count + 1}]`,
	)
	first.Spec.Variables = []admissionregistrationv1.Variable{{Name: "count", Expression: "object.count"}}
	first.Spec.AuditAnnotations = []admissionregistrationv1.AuditAnnotation{
		{Key: "count", ValueExpression: "string(object.count)"},
		{Key: "omitted", ValueExpression: "null"},
	}
	second := jsonMutationPolicy("a-second", `[JSONPatch{op:"add",path:"/result",value:object.count * 2}]`)
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{first, second}, nil)
	require.NoError(t, err)
	response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, `{"count":3,"result":6}`, compactJSON(t, response.Document))
	require.Equal(t, map[string]string{"count": "3"}, response.Policies[0].AuditAnnotations)
	require.Equal(t, "z-first", response.Policies[0].Policy)
}

func TestJSONEnginePerPolicyDocuments(t *testing.T) {
	t.Parallel()
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{
		jsonMutationPolicy("first", `[JSONPatch{op:"add",path:"/a",value:1}]`),
		jsonMutationPolicy("skipped", `[JSONPatch{op:"test",path:"/a",value:2},JSONPatch{op:"add",path:"/never",value:true}]`),
		jsonMutationPolicy("second", `[JSONPatch{op:"add",path:"/b",value:2}]`),
	}, nil)
	require.NoError(t, err)
	response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Len(t, response.Policies, 3)
	require.Equal(t, `{"a":1}`, compactJSON(t, response.Policies[0].Document))
	require.Equal(t, JSONPolicySkipped, response.Policies[1].Status)
	require.Equal(t, `{"a":1}`, compactJSON(t, response.Policies[1].Document))
	require.Equal(t, `{"a":1,"b":2}`, compactJSON(t, response.Policies[2].Document))
	require.Equal(t, `{"a":1,"b":2}`, compactJSON(t, response.Document))
	// Snapshots are independent buffers: mutating one must not affect another.
	response.Policies[0].Document[1] = 'x'
	require.Equal(t, `{"a":1}`, compactJSON(t, response.Policies[1].Document))

	failing, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{
		jsonMutationPolicy("first", `[JSONPatch{op:"add",path:"/a",value:1}]`),
		jsonMutationPolicy("broken", `[JSONPatch{op:"remove",path:"/missing"}]`),
	}, nil)
	require.NoError(t, err)
	response, err = failing.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.Error(t, err)
	require.Nil(t, response.Document)
	require.Equal(t, `{"a":1}`, compactJSON(t, response.Policies[0].Document))
	require.Nil(t, response.Policies[1].Document)
}

func TestJSONEngineSkipAndAtomicity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		policy     *policiesv1beta1.MutatingPolicy
		exceptions []*policiesv1beta1.PolicyException
		wantError  string
	}{
		{
			name: "failed test rolls back all mutations",
			policy: jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/a",value:1}]`,
				`[JSONPatch{op:"test",path:"/a",value:2}]`),
		},
		{
			name: "false match",
			policy: func() *policiesv1beta1.MutatingPolicy {
				policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/a",value:1}]`)
				policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Expression: "false"}}
				return policy
			}(),
		},
		{
			name:   "exception",
			policy: jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/a",value:1}]`),
			exceptions: []*policiesv1beta1.PolicyException{{
				ObjectMeta: metav1.ObjectMeta{Name: "exempt"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: "test", Kind: "MutatingPolicy"}},
					MatchConditions: []admissionregistrationv1.MatchCondition{{Expression: "object.exempt == true"}},
				},
			}},
		},
		{
			name:      "error rolls back all mutations",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/a",value:1}]`, `[JSONPatch{op:"remove",path:"/missing"}]`),
			wantError: "mutation 1",
		},
		{
			name:      "missing from",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"copy",path:"/copy"}]`),
			wantError: "requires from",
		},
		{
			name:      "root removal",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"remove",path:""}]`),
			wantError: "cannot remove the document root",
		},
		{
			name:      "invalid pointer",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/~2",value:1}]`),
			wantError: "invalid escape",
		},
		{
			name:      "negative index",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/array/-1",value:1}]`),
			wantError: "operation 0",
		},
		{
			name:      "root move to descendant",
			policy:    jsonMutationPolicy("test", `[JSONPatch{op:"move",from:"",path:"/copy"}]`),
			wantError: "descendant",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{tt.policy}, tt.exceptions)
			require.NoError(t, err)
			input := json.RawMessage(`{"exempt":true,"array":[0]}`)
			response, err := engine.HandleJSON(context.Background(), input)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, response.Document)
				require.Equal(t, JSONPolicyError, response.Policies[0].Status)
			} else {
				require.NoError(t, err)
				require.Equal(t, input, response.Document)
				require.Equal(t, JSONPolicySkipped, response.Policies[0].Status)
			}
			require.Equal(t, `{"exempt":true,"array":[0]}`, string(input))
		})
	}
}

func TestJSONEngineInvalidPolicies(t *testing.T) {
	t.Parallel()
	for _, expression := range []string{
		`request.operation == "CREATE"`,
		`[JSONPatch{op:"add",path:"/x",value:namespace.metadata.name}]`,
		`[JSONPatch{op:"add",path:"/x",value:resource.Get("v1","pods","default","x")}]`,
		`[JSONPatch{op:"add",path:"/x",value:Object{}}]`,
		`[1]`,
		`[JSONPatch{op:"add",path:"/x",value:variables.missing}]`,
		`[JSONPatch{op:"add",path:"/x",value:1,unexpected:true}]`,
	} {
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			_, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", expression)}, nil)
			require.Error(t, err)
		})
	}
	// The JSON environment is pure: libraries that reach the network, cluster
	// state or registries must stay undeclared so a JSON policy cannot be used
	// for SSRF or data exfiltration.
	for _, expression := range []string{
		`[JSONPatch{op:"add",path:"/x",value:http.Get("https://example.invalid/", {"Accept":"application/json"})}]`,
		`[JSONPatch{op:"add",path:"/x",value:http.Post("https://example.invalid/", {}, {})}]`,
		`[JSONPatch{op:"add",path:"/x",value:globalContext.Get("entry","")}]`,
		`[JSONPatch{op:"add",path:"/x",value:image.GetMetadata("nginx")}]`,
		`[JSONPatch{op:"add",path:"/x",value:resource.List("v1","pods","default")}]`,
		`[JSONPatch{op:"add",path:"/x",value:oldObject}]`,
	} {
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			_, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", expression)}, nil)
			require.ErrorContains(t, err, "undeclared reference")
		})
	}
	for _, setting := range []string{"mode", "apply", "autogen", "map", "target", "ssa", "reinvocation", "empty"} {
		t.Run(setting, func(t *testing.T) {
			t.Parallel()
			policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/x",value:1}]`)
			switch setting {
			case "mode":
				policy.Spec.EvaluationConfiguration.Mode = policieskyvernoio.EvaluationModeKubernetes
			case "apply":
				policy.Spec.Mutations[0].PatchType = admissionregistrationv1alpha1.PatchTypeApplyConfiguration
			case "autogen":
				policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
					PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{Controllers: []string{"deployments"}},
				}
			case "map":
				enabled := true
				policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
					MutatingAdmissionPolicy: &policiesv1beta1.MAPGenerationConfiguration{Enabled: &enabled},
				}
			case "target":
				policy.Spec.TargetMatchConstraints = &policiesv1beta1.TargetMatchConstraints{}
			case "ssa":
				policy.Spec.EvaluationConfiguration.UseServerSideApply = true
			case "reinvocation":
				policy.Spec.ReinvocationPolicy = admissionregistrationv1.IfNeededReinvocationPolicy
			case "empty":
				policy.Spec.Mutations = nil
			}
			_, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy}, nil)
			require.Error(t, err)
		})
	}
}

func TestJSONEngineDefaultedFieldsAccepted(t *testing.T) {
	t.Parallel()
	// Fields the API server may default or users may leave in place must not
	// reject an otherwise valid JSON policy.
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/x",value:1}]`)
	enabled := true
	disabled := false
	policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
		PodControllers:          &policiesv1beta1.PodControllersGenerationConfiguration{},
		MutatingAdmissionPolicy: &policiesv1beta1.MAPGenerationConfiguration{Enabled: &disabled},
	}
	policy.Spec.EvaluationConfiguration.Admission = &policiesv1beta1.AdmissionConfiguration{Enabled: &enabled}
	policy.Spec.EvaluationConfiguration.Background = &policiesv1beta1.BackgroundConfiguration{Enabled: &enabled}
	policy.Spec.EvaluationConfiguration.MutateExistingConfiguration = &policiesv1beta1.MutateExistingConfiguration{Enabled: &disabled}
	policy.Spec.ReinvocationPolicy = admissionregistrationv1.NeverReinvocationPolicy
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, `{"x":1}`, compactJSON(t, response.Document))
}

func TestJSONEngineTestOperation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, expression string
		expected                string // empty means the policy is skipped
		wantError               string
	}{
		{"integer equals double", `{"n":1}`, `[JSONPatch{op:"test",path:"/n",value:1.0},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":1,"ok":true}`, ""},
		{"double equals integer literal", `{"n":1.0}`, `[JSONPatch{op:"test",path:"/n",value:1},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":1.0,"ok":true}`, ""},
		{"exponent equals integer", `{"n":1e2}`, `[JSONPatch{op:"test",path:"/n",value:100},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":1e2,"ok":true}`, ""},
		{"fractional decimal equals double", `{"n":0.1}`, `[JSONPatch{op:"test",path:"/n",value:0.1},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":0.1,"ok":true}`, ""},
		{"fractional decimal from object", `{"m":[0.1],"n":0.1}`, `[JSONPatch{op:"test",path:"/m/0",value:object.n},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"m":[0.1],"n":0.1,"ok":true}`, ""},
		{"fractional decimals differ", `{"n":0.1}`, `[JSONPatch{op:"test",path:"/n",value:0.2},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"large integer exact", `{"n":9007199254740993}`, `[JSONPatch{op:"test",path:"/n",value:9007199254740993},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":9007199254740993,"ok":true}`, ""},
		{"large integer not rounded", `{"n":9007199254740993}`, `[JSONPatch{op:"test",path:"/n",value:9007199254740992},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"numbers differ", `{"n":1}`, `[JSONPatch{op:"test",path:"/n",value:2},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"string is not number", `{"n":"1"}`, `[JSONPatch{op:"test",path:"/n",value:1},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"missing member is a failed test", `{}`, `[JSONPatch{op:"test",path:"/missing",value:1},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"missing member is not null", `{}`, `[JSONPatch{op:"test",path:"/missing",value:null},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"missing container is a failed test", `{}`, `[JSONPatch{op:"test",path:"/a/b",value:1},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"null member equals null", `{"n":null}`, `[JSONPatch{op:"test",path:"/n",value:null},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":null,"ok":true}`, ""},
		{"object order independent", `{"o":{"a":1,"b":[1,2.0]}}`, `[JSONPatch{op:"test",path:"/o",value:{"b":[1.0,2],"a":1}},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"o":{"a":1,"b":[1,2.0]},"ok":true}`, ""},
		{"object subset is not equal", `{"o":{"a":1,"b":2}}`, `[JSONPatch{op:"test",path:"/o",value:{"a":1}},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"array order matters", `{"l":[1,2]}`, `[JSONPatch{op:"test",path:"/l",value:[2,1]},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"array element", `{"l":[1,2]}`, `[JSONPatch{op:"test",path:"/l/1",value:2},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"l":[1,2],"ok":true}`, ""},
		{"array index out of range", `{"l":[1,2]}`, `[JSONPatch{op:"test",path:"/l/2",value:2},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"array dash is not a location", `{"l":[1,2]}`, `[JSONPatch{op:"test",path:"/l/-",value:2},JSONPatch{op:"add",path:"/ok",value:true}]`, ``, ""},
		{"non-canonical index rejected", `{"l":[1,2]}`, `[JSONPatch{op:"test",path:"/l/01",value:2}]`, ``, "non-canonical array index"},
		{"test does not mutate", `{"n":1}`, `[JSONPatch{op:"test",path:"/n",value:1}]`, `{"n":1}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", tt.expression)}, nil)
			require.NoError(t, err)
			response, err := engine.HandleJSON(context.Background(), json.RawMessage(tt.input))
			switch {
			case tt.wantError != "":
				require.ErrorContains(t, err, tt.wantError)
				require.Equal(t, JSONPolicyError, response.Policies[0].Status)
			case tt.expected == "":
				require.NoError(t, err)
				require.Equal(t, JSONPolicySkipped, response.Policies[0].Status)
				require.Equal(t, tt.input, string(response.Document))
			default:
				require.NoError(t, err)
				require.Equal(t, tt.expected, compactJSON(t, response.Document))
			}
		})
	}
}

func TestJSONEngineCanonicalArrayIndexes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, expression string
		expected         string
		wantError        string
	}{
		{"leading zero", `[JSONPatch{op:"replace",path:"/l/01",value:9}]`, "", "non-canonical array index"},
		{"plus sign", `[JSONPatch{op:"add",path:"/l/+1",value:9}]`, "", "non-canonical array index"},
		{"negative zero", `[JSONPatch{op:"remove",path:"/l/-0"}]`, "", "non-canonical array index"},
		{"non-canonical from", `[JSONPatch{op:"copy",from:"/l/00",path:"/copy"}]`, "", "non-canonical array index"},
		{"nested", `[JSONPatch{op:"replace",path:"/l/2/01",value:9}]`, "", "non-canonical array index"},
		{"object key that looks numeric", `[JSONPatch{op:"add",path:"/o/01",value:9}]`, `{"l":[1,2,[3,4]],"o":{"01":9}}`, ""},
		{"canonical index", `[JSONPatch{op:"replace",path:"/l/1",value:9}]`, `{"l":[1,9,[3,4]],"o":{}}`, ""},
		{"append", `[JSONPatch{op:"add",path:"/l/-",value:9}]`, `{"l":[1,2,[3,4],9],"o":{}}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", tt.expression)}, nil)
			require.NoError(t, err)
			response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{"l":[1,2,[3,4]],"o":{}}`))
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, response.Document)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, compactJSON(t, response.Document))
		})
	}
}

func TestJSONEngineCostBudget(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("cost", `[JSONPatch{op:"add",path:"/0",value:1}]`)
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
		Expression: `object.all(x, object.all(y, object.all(z, x == y && y == z)))`,
	}}
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	response, err := engine.HandleJSON(context.Background(), json.RawMessage("["+strings.Repeat("0,", 255)+"0]"))
	require.ErrorContains(t, err, "cost limit")
	require.Nil(t, response.Document)
}

func TestJSONEnginePartialException(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/allowed",value:exceptions.allowedValues}]`)
	exception := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:    []policiesv1beta1.PolicyRef{{Name: "test", Kind: "MutatingPolicy"}},
			AllowedValues: []string{"one", "two"},
		},
	}
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy}, []*policiesv1beta1.PolicyException{exception})
	require.NoError(t, err)
	exception.Spec.AllowedValues[0] = "changed"
	response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, `{"allowed":["one","two"]}`, compactJSON(t, response.Document))
	require.Equal(t, JSONPolicyApplied, response.Policies[0].Status)
}

func TestJSONEngineConstructionAndFailure(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/ok",value:true}]`)
	for _, policies := range [][]policiesv1beta1.MutatingPolicyLike{
		{nil},
		{(*policiesv1beta1.MutatingPolicy)(nil)},
		{policy, policy},
	} {
		_, err := NewJSONEngine(policies, nil)
		require.Error(t, err)
	}
	empty, err := NewJSONEngine(nil, nil)
	require.NoError(t, err)
	response, err := empty.HandleJSON(context.Background(), json.RawMessage(`null`))
	require.NoError(t, err)
	require.Equal(t, "null", string(response.Document))

	fail := jsonMutationPolicy("fail", `[JSONPatch{op:"remove",path:"/missing"}]`)
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy, fail}, nil)
	require.NoError(t, err)
	response, err = engine.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.ErrorContains(t, err, "JSON policy fail")
	require.Nil(t, response.Document)
	require.Len(t, response.Policies, 2)
	require.Equal(t, JSONPolicyApplied, response.Policies[0].Status)
	require.Equal(t, JSONPolicyError, response.Policies[1].Status)
	require.Error(t, response.Policies[1].Error)
}

func TestJSONEngineLimitsAndCancellation(t *testing.T) {
	t.Parallel()
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/ok",value:true}]`)}, nil)
	require.NoError(t, err)
	identity, err := NewJSONEngine(nil, nil)
	require.NoError(t, err)
	boundary := json.RawMessage(`"` + strings.Repeat("x", compiler.MaxJSONDocumentBytes-2) + `"`)
	unchanged, err := identity.HandleJSON(context.Background(), boundary)
	require.NoError(t, err)
	require.Len(t, unchanged.Document, compiler.MaxJSONDocumentBytes)
	for _, input := range []string{"", "{} {}", `{"n":9223372036854775808}`, `{"n":1e999}`, `"` + strings.Repeat("x", compiler.MaxJSONDocumentBytes) + `"`} {
		response, err := engine.HandleJSON(context.Background(), json.RawMessage(input))
		require.Error(t, err)
		require.Nil(t, response.Document)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err := engine.HandleJSON(ctx, json.RawMessage(`{}`))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, response.Document)

	limited, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("many",
		`object.map(x, JSONPatch{op:"add",path:"/0",value:x})`)}, nil)
	require.NoError(t, err)
	atLimit, err := limited.HandleJSON(context.Background(), json.RawMessage("["+strings.Repeat("0,", compiler.MaxJSONPatchOperations-1)+"0]"))
	require.NoError(t, err)
	require.Equal(t, JSONPolicyApplied, atLimit.Policies[0].Status)
	response, err = limited.HandleJSON(context.Background(), json.RawMessage("["+strings.Repeat("0,", compiler.MaxJSONPatchOperations)+"0]"))
	require.ErrorContains(t, err, "patch operations")
	require.Nil(t, response.Document)

	overflow, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("uint", `[JSONPatch{op:"add",path:"/u",value:9223372036854775808u}]`)}, nil)
	require.NoError(t, err)
	response, err = overflow.HandleJSON(context.Background(), json.RawMessage(`{}`))
	require.ErrorContains(t, err, "outside the int64 range")
	require.Nil(t, response.Document)

	large, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("large", `[JSONPatch{op:"copy",from:"/x",path:"/y"}]`)}, nil)
	require.NoError(t, err)
	response, err = large.HandleJSON(context.Background(), json.RawMessage(`{"x":"`+strings.Repeat("x", compiler.MaxJSONDocumentBytes/2)+`"}`))
	require.ErrorContains(t, err, "exceeds")
	require.Nil(t, response.Document)
}

func TestJSONEngineConcurrent(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/copy",value:variables.id}]`)
	policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "id", Expression: "object.id"}}
	engine, err := NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	policy.Spec.Mutations[0].JSONPatch.Expression = "invalid"
	var wait sync.WaitGroup
	for range 16 {
		wait.Go(func() {
			response, err := engine.HandleJSON(context.Background(), json.RawMessage(`{"id":7}`))
			var document struct{ ID, Copy int }
			decodeErr := json.Unmarshal(response.Document, &document)
			if err != nil || decodeErr != nil || document.ID != 7 || document.Copy != 7 {
				t.Errorf("unexpected response %s, error %v", response.Document, err)
			}
		})
	}
	wait.Wait()
}
