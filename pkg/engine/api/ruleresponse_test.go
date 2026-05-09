package api

import (
	"errors"
	"testing"
)

func TestRuleResponse_String(t *testing.T) {
	type fields struct {
		Name    string
		Type    RuleType
		Message string
		Status  RuleStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{
			Name:    "test-mutation",
			Type:    Mutation,
			Message: "message",
		},
		want: "rule test-mutation (Mutation): message",
	}, {
		fields: fields{
			Name:    "test-validation",
			Type:    Validation,
			Message: "message",
		},
		want: "rule test-validation (Validation): message",
	}, {
		fields: fields{
			Name:    "test-generation",
			Type:    Generation,
			Message: "message",
		},
		want: "rule test-generation (Generation): message",
	}, {
		fields: fields{
			Name:    "test-image-verify",
			Type:    ImageVerify,
			Message: "message",
		},
		want: "rule test-image-verify (ImageVerify): message",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := NewRuleResponse(
				tt.fields.Name,
				tt.fields.Type,
				tt.fields.Message,
				tt.fields.Status,
				nil,
			)
			if got := rr.String(); got != tt.want {
				t.Errorf("RuleResponse.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleError_IncludesErrorDetails(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  RuleStatus
		wantMessage string
	}{
		{
			name:        "non-nil error includes details in message",
			err:         errors.New("something went wrong"),
			wantStatus:  RuleStatusError,
			wantMessage: "error: something went wrong",
		},
		{
			name:        "nil error produces message without details",
			err:         nil,
			wantStatus:  RuleStatusError,
			wantMessage: "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := RuleError("test-rule", Validation, "error", tt.err, nil)
			if resp.Status() != tt.wantStatus {
				t.Errorf("RuleError().Status() = %v, want %v", resp.Status(), tt.wantStatus)
			}
			if resp.Message() != tt.wantMessage {
				t.Errorf("RuleError().Message() = %v, want %v", resp.Message(), tt.wantMessage)
			}
		})
	}
}

func TestRuleResponse_HasStatus(t *testing.T) {
	type fields struct {
		Name    string
		Type    RuleType
		Message string
		Status  RuleStatus
	}
	type args struct {
		status []RuleStatus
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{{
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusError},
		},
		want: false,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusError, RuleStatusPass, RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{},
		},
		want: false,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRuleResponse(
				tt.fields.Name,
				tt.fields.Type,
				tt.fields.Message,
				tt.fields.Status,
				nil,
			)
			if got := r.HasStatus(tt.args.status...); got != tt.want {
				t.Errorf("RuleResponse.HasStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
