package api

import (
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestImageVerificationMetadata_IsVerified(t *testing.T) {
	type fields struct {
		Data map[string]ImageVerificationMetadataStatus
	}
	type args struct {
		image string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{{
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			image: "test",
		},
		want: true,
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			image: "test2",
		},
		want: false,
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test2": ImageVerificationFail,
			},
		},
		args: args{
			image: "test2",
		},
		want: false,
	}, {
		args: args{
			image: "test2",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivm := &ImageVerificationMetadata{
				Data: tt.fields.Data,
			}
			if got := ivm.IsVerified(tt.args.image); got != tt.want {
				t.Errorf("ImageVerificationMetadata.IsVerified() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageVerificationMetadata_Add(t *testing.T) {
	type fields struct {
		Data map[string]ImageVerificationMetadataStatus
	}
	type args struct {
		image    string
		verified ImageVerificationMetadataStatus
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *ImageVerificationMetadata
	}{{
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			image:    "test",
			verified: ImageVerificationFail,
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationFail,
			},
		},
	}, {
		args: args{
			image:    "test",
			verified: ImageVerificationFail,
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationFail,
			},
		},
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			image:    "test2",
			verified: ImageVerificationFail,
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test":  ImageVerificationPass,
				"test2": ImageVerificationFail,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivm := &ImageVerificationMetadata{
				Data: tt.fields.Data,
			}
			ivm.Add(tt.args.image, tt.args.verified)
			if !reflect.DeepEqual(ivm, tt.want) {
				t.Errorf("ImageVerificationMetadata.Add() = %v, want %v", ivm, tt.want)
			}
		})
	}
}

func TestParseImageMetadata(t *testing.T) {
	type args struct {
		jsonData string
	}
	tests := []struct {
		name    string
		args    args
		want    *ImageVerificationMetadata
		wantErr bool
	}{{
		args: args{
			jsonData: `"error"`,
		},
		wantErr: true,
	}, {
		args: args{
			jsonData: `{"test":"pass"}`,
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
	}, {
		args: args{
			jsonData: `{"test":"pass","test2":"fail"}`,
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test":  ImageVerificationPass,
				"test2": ImageVerificationFail,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageMetadata(tt.args.jsonData)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseImageMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageVerificationMetadata_IsEmpty(t *testing.T) {
	type fields struct {
		Data map[string]ImageVerificationMetadataStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationFail,
			},
		},
		want: false,
	}, {
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivm := &ImageVerificationMetadata{
				Data: tt.fields.Data,
			}
			if got := ivm.IsEmpty(); got != tt.want {
				t.Errorf("ImageVerificationMetadata.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageVerificationMetadata_Merge(t *testing.T) {
	type fields struct {
		Data map[string]ImageVerificationMetadataStatus
	}
	type args struct {
		other ImageVerificationMetadata
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *ImageVerificationMetadata
	}{{
		want: &ImageVerificationMetadata{},
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			other: ImageVerificationMetadata{
				Data: map[string]ImageVerificationMetadataStatus{
					"test": ImageVerificationFail,
				},
			},
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationFail,
			},
		},
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			other: ImageVerificationMetadata{
				Data: map[string]ImageVerificationMetadataStatus{
					"test2": ImageVerificationFail,
				},
			},
		},
		want: &ImageVerificationMetadata{
			Data: map[string]ImageVerificationMetadataStatus{
				"test":  ImageVerificationPass,
				"test2": ImageVerificationFail,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivm := &ImageVerificationMetadata{
				Data: tt.fields.Data,
			}
			ivm.Merge(tt.args.other)
			if !reflect.DeepEqual(ivm, tt.want) {
				t.Errorf("ImageVerificationMetadata.Merge() = %v, want %v", ivm, tt.want)
			}
		})
	}
}

func Test_makeAnnotationKeyForJSONPatch(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{{
		want: "/metadata/annotations/kyverno.io~1verify-images",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeAnnotationKeyForJSONPatch(); got != tt.want {
				t.Errorf("makeAnnotationKeyForJSONPatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageVerificationMetadata_Patches(t *testing.T) {
	type fields struct {
		Data map[string]ImageVerificationMetadataStatus
	}
	type args struct {
		hasAnnotations bool
		log            logr.Logger
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{{
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			hasAnnotations: false,
			log:            logr.Discard(),
		},
		want: []string{
			`{"op":"add","path":"/metadata/annotations","value":{}}`,
			`{"op":"add","path":"/metadata/annotations/kyverno.io~1verify-images","value":"{\"test\":\"pass\"}"}`,
		},
	}, {
		fields: fields{
			Data: map[string]ImageVerificationMetadataStatus{
				"test": ImageVerificationPass,
			},
		},
		args: args{
			hasAnnotations: true,
			log:            logr.Discard(),
		},
		want: []string{
			`{"op":"add","path":"/metadata/annotations/kyverno.io~1verify-images","value":"{\"test\":\"pass\"}"}`,
		},
	}, {
		args: args{
			hasAnnotations: true,
			log:            logr.Discard(),
		},
		want: []string{
			`{"op":"add","path":"/metadata/annotations/kyverno.io~1verify-images","value":"null"}`,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivm := &ImageVerificationMetadata{
				Data: tt.fields.Data,
			}
			got, err := ivm.Patches(tt.args.hasAnnotations, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImageVerificationMetadata.Patches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, len(got), len(tt.want))
			for i := range got {
				assert.Equal(t, got[i].Json(), tt.want[i])
			}
		})
	}
}
