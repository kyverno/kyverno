package api

import "testing"

func TestResourceSpec_GetKey(t *testing.T) {
	type fields struct {
		Kind       string
		APIVersion string
		Namespace  string
		Name       string
		UID        string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{
			Kind:      "Pod",
			Namespace: "test",
			Name:      "nignx",
		},
		want: "Pod/test/nignx",
	}, {
		fields: fields{
			Kind: "ClusterRole",
			Name: "admin",
		},
		want: "ClusterRole//admin",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := ResourceSpec{
				Kind:       tt.fields.Kind,
				APIVersion: tt.fields.APIVersion,
				Namespace:  tt.fields.Namespace,
				Name:       tt.fields.Name,
				UID:        tt.fields.UID,
			}
			if got := rs.String(); got != tt.want {
				t.Errorf("ResourceSpec.GetKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
