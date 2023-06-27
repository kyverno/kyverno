package api

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

type dummyResolver struct {
	err error
	cm  *corev1.ConfigMap
}

func (c dummyResolver) Get(context.Context, string, string) (*corev1.ConfigMap, error) {
	return c.cm, c.err
}

func TestNewNamespacedResourceResolver(t *testing.T) {
	type args struct {
		resolvers []ConfigmapResolver
	}
	tests := []struct {
		name    string
		args    args
		want    ConfigmapResolver
		wantErr bool
	}{{
		name:    "nil shoud return an error",
		wantErr: true,
	}, {
		name:    "empty list shoud return an error",
		args:    args{[]ConfigmapResolver{}},
		wantErr: true,
	}, {
		name:    "one nil in the list shoud return an error",
		args:    args{[]ConfigmapResolver{dummyResolver{}, nil}},
		wantErr: true,
	}, {
		name: "no nil",
		args: args{[]ConfigmapResolver{dummyResolver{}, dummyResolver{}, dummyResolver{}}},
		want: namespacedResourceResolverChain[*corev1.ConfigMap]{dummyResolver{}, dummyResolver{}, dummyResolver{}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewNamespacedResourceResolver(tt.args.resolvers...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResolverChain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResolverChain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespacedResourceResolverChain_Get(t *testing.T) {
	type fields struct {
		resolvers []ConfigmapResolver
	}
	type args struct {
		namespace string
		name      string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
		wantCm  *corev1.ConfigMap
	}{{
		name: "Test0",
		fields: fields{
			resolvers: []ConfigmapResolver{
				dummyResolver{},
				dummyResolver{},
				dummyResolver{},
			},
		},
	}, {
		name: "Test1",
		fields: fields{
			resolvers: []ConfigmapResolver{
				dummyResolver{
					err: errors.New("1"),
				},
				dummyResolver{
					err: errors.New("2"),
				},
				dummyResolver{
					err: errors.New("3"),
				},
			},
		},
		wantErr: errors.New("3"),
	}, {
		name: "Test2",
		fields: fields{
			resolvers: []ConfigmapResolver{
				dummyResolver{
					err: errors.New("1"),
				},
				dummyResolver{},
				dummyResolver{
					err: errors.New("3"),
				},
			},
		},
	}, {
		name: "Test3",
		fields: fields{
			resolvers: []ConfigmapResolver{
				dummyResolver{
					err: errors.New("1"),
				},
				dummyResolver{
					err: errors.New("2"),
				},
				dummyResolver{},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, _ := NewNamespacedResourceResolver(tt.fields.resolvers...)
			got, err := resolver.Get(context.TODO(), tt.args.namespace, tt.args.name)
			if !checkError(tt.wantErr, err) {
				t.Errorf("ConfigmapResolver.Get() %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}

			if !reflect.DeepEqual(got, tt.wantCm) {
				t.Errorf("ConfigmapResolver.Get() = %v, want %v", got, tt.wantCm)
			}
		})
	}
}

func checkError(wantErr, err error) bool {
	if wantErr != nil {
		if err == nil {
			return false
		}
		return wantErr.Error() == err.Error()
	}
	return err == nil
}
