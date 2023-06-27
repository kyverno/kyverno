package cleanup

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var jp = jmespath.New(config.NewDefaultConfiguration(false))

func Test_checkCondition(t *testing.T) {
	ctx := enginecontext.NewContext(jp)
	ctx.AddResource(map[string]interface{}{
		"name": "dummy",
	})
	type args struct {
		logger    logr.Logger
		ctx       enginecontext.Interface
		condition kyvernov2beta1.Condition
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		name: "basic",
		args: args{
			logger: logging.GlobalLogger(),
			ctx:    ctx,
			condition: kyvernov2beta1.Condition{
				RawKey: &v1.JSON{
					Raw: []byte(`"{{ request.object.name }}"`),
				},
				Operator: kyvernov2beta1.ConditionOperators["Equals"],
				RawValue: &v1.JSON{
					Raw: []byte(`"dummy"`),
				},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkCondition(tt.args.logger, tt.args.ctx, tt.args.condition)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkCondition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
