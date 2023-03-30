package resolvers

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
)

func TestGetCacheSelector(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{{
		name: "ok",
		want: LabelCacheKey,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCacheSelector()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCacheSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want) {
				t.Errorf("GetCacheSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCacheInformerFactory(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		client  kubernetes.Interface
		wantErr bool
	}{{
		name:    "nil client",
		wantErr: true,
		client:  nil,
	}, {
		name:    "ok",
		wantErr: false,
		client:  newEmptyFakeClient(),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetCacheInformerFactory(tt.client, 10*time.Minute)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCacheInformerFactor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
