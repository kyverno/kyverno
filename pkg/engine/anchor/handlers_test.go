package anchor

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNegationHandler_Handle(t *testing.T) {
	tests := []struct {
		name         string
		resourceMap  map[string]interface{}
		expectedPath string
		expectedErr  error
	}{
		{
			name: "should return error when key exists in resource",
			resourceMap: map[string]interface{}{
				"testKey": "value",
			},
			expectedPath: "testPath/testKey/",
			expectedErr:  newNegationAnchorError("testPath/testKey/ is not allowed"),
		},
		{
			name:         "should pass when key does not exist in resource",
			resourceMap:  map[string]interface{}{},
			expectedPath: "",
			expectedErr:  nil,
		},
	}

	loggerFunc := func(log logr.Logger, resourceElement interface{}, patternElement interface{},
		originPattern interface{}, path string, ac *AnchorMap) (string, error) {
		return "", nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchor := anchor{
				modifier: Negation,
				key:      "testKey",
			}

			handler := newNegationHandler(anchor, nil, "testPath/")

			path, err := handler.Handle(loggerFunc, tt.resourceMap, "testPath/", &AnchorMap{})

			assert.Equal(t, tt.expectedPath, path)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestEqualityHandler_Handle(t *testing.T) {
	tests := []struct {
		name         string
		resourceMap  map[string]interface{}
		expectedPath string
		expectedErr  string
	}{
		{
			name: "should pass when anchor key exists and matches pattern",
			resourceMap: map[string]interface{}{
				"testKey": "value",
			},
			expectedPath: "",
			expectedErr:  "",
		},
		{
			name: "should retain error when anchor key exists but does not match pattern",
			resourceMap: map[string]interface{}{
				"testKey": "wrongValue",
			},
			expectedPath: "testPath/testKey/",
			expectedErr:  "value mismatch at testPath/testKey/",
		},
		{
			name:         "should pass when anchor key does not exist",
			resourceMap:  map[string]interface{}{},
			expectedPath: "",
			expectedErr:  "",
		},
	}

	loggerFunc := func(
		log logr.Logger, resourceElement interface{}, patternElement interface{},
		originPattern interface{}, path string, ac *AnchorMap) (string, error) {
		// If the resource and pattern elements don't match, return the path with an error
		if resourceElement != patternElement {
			return path, fmt.Errorf("value mismatch at %s", path)
		}
		return "", nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchor := anchor{
				modifier: Equality,
				key:      "testKey",
			}

			handler := newEqualityHandler(anchor, "value", "testPath/")
			path, err := handler.Handle(loggerFunc, tt.resourceMap, "testPath/", &AnchorMap{})

			if err != nil {
				assert.Equal(t, tt.expectedErr, err.Error())
			} else {
				assert.Equal(t, tt.expectedErr, "")
			}

			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// MockResourceElementHandler is a mocked implementation of the resourceElementHandler
type MockResourceElementHandler struct {
	mock.Mock
}

func (m *MockResourceElementHandler) Handle(
	log logr.Logger,
	resourceElement interface{},
	patternElement interface{},
	originPattern interface{},
	path string,
	ac *AnchorMap,
) (string, error) {
	args := m.Called(log, resourceElement, patternElement, originPattern, path, ac)
	return args.String(0), args.Error(1)
}
