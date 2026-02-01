package imagedataloader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		wantErr    bool
		wantReg    string
		wantRepo   string
		wantTag    string
		wantDigest string
	}{
		// Standard images with tags
		{"simple with tag", "nginx:1.19", false, "index.docker.io", "library/nginx", "1.19", ""},
		{"repo with tag", "myrepo/myimage:v1.0", false, "index.docker.io", "myrepo/myimage", "v1.0", ""},
		{"full registry", "gcr.io/project/image:latest", false, "gcr.io", "project/image", "latest", ""},
		{"private registry with port", "localhost:5000/myimage:dev", false, "localhost:5000", "myimage", "dev", ""},

		// Images with digest
		{"with digest", "nginx@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false, "index.docker.io", "library/nginx", "", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"registry with digest", "gcr.io/proj/img@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false, "gcr.io", "proj/img", "", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},

		// Default tag (latest)
		{"no tag defaults", "nginx", false, "index.docker.io", "library/nginx", "latest", ""},
		{"repo no tag", "myrepo/app", false, "index.docker.io", "myrepo/app", "latest", ""},

		// Invalid images
		{"empty string", "", true, "", "", "", ""},
		{"invalid format", "::invalid::", true, "", "", "", ""},
		{"only colon", ":", true, "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseImageReference(tt.image)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.image, ref.Image)
			assert.Equal(t, tt.wantReg, ref.Registry)
			assert.Equal(t, tt.wantRepo, ref.Repository)
			assert.Equal(t, tt.wantTag, ref.Tag)
			assert.Equal(t, tt.wantDigest, ref.Digest)
		})
	}
}

func TestParseImageReference_Identifier(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		wantIdent string
	}{
		{"tag identifier", "nginx:1.19", "1.19"},
		{"digest identifier", "nginx@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"default identifier", "nginx", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseImageReference(tt.image)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantIdent, ref.Identifier)
		})
	}
}
