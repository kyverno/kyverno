package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// mockDeleter is a mock for the Deleter interface.
type mockDeleter struct {
	deleted []string
}

func (d *mockDeleter) Delete(ctx context.Context, name string, options metav1.DeleteOptions) error {
	d.deleted = append(d.deleted, name)
	return nil
}

// mockObject is a mock for metav1.Object.
type mockObject struct {
	metav1.ObjectMeta
}

func (o *mockObject) DeepCopy() *mockObject {
	return &mockObject{
		ObjectMeta: *o.ObjectMeta.DeepCopy(),
	}
}

// mockGetter is a mock for the Getter interface.
type mockGetter struct {
	obj *mockObject
	err error
}

func (g *mockGetter) Get(name string) (*mockObject, error) {
	if g.err != nil {
		return nil, g.err
	}
	if g.obj != nil && g.obj.Name == name {
		return g.obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
}

// mockSetter is a mock for the Setter interface.
type mockSetter struct {
	created *mockObject
	updated *mockObject
	err     error
}

func (s *mockSetter) Create(ctx context.Context, obj *mockObject, options metav1.CreateOptions) (*mockObject, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = obj
	return obj, nil
}

func (s *mockSetter) Update(ctx context.Context, obj *mockObject, options metav1.UpdateOptions) (*mockObject, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.updated = obj
	return obj, nil
}

// mockUpdateClient is a mock for the UpdateClient interface.
type mockUpdateClient struct {
	updated *mockObject
	err     error
}

func (c *mockUpdateClient) Update(ctx context.Context, obj *mockObject, options metav1.UpdateOptions) (*mockObject, error) {
	if c.err != nil {
		return nil, c.err
	}
	c.updated = obj
	return obj, nil
}

// mockObjectStatusClient is a mock for the ObjectStatusClient interface.
type mockObjectStatusClient struct {
	updated *mockObject
	err     error
}

func (c *mockObjectStatusClient) UpdateStatus(ctx context.Context, obj *mockObject, options metav1.UpdateOptions) (*mockObject, error) {
	if c.err != nil {
		return nil, c.err
	}
	c.updated = obj
	return obj, nil
}

func (c *mockObjectStatusClient) Create(context.Context, *mockObject, metav1.CreateOptions) (*mockObject, error) {
	panic("not implemented")
}
func (c *mockObjectStatusClient) Get(context.Context, string, metav1.GetOptions) (*mockObject, error) {
	panic("not implemented")
}
func (c *mockObjectStatusClient) Update(context.Context, *mockObject, metav1.UpdateOptions) (*mockObject, error) {
	panic("not implemented")
}
func (c *mockObjectStatusClient) Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*mockObject, error) {
	panic("not implemented")
}
func (c *mockObjectStatusClient) Delete(context.Context, string, metav1.DeleteOptions) error {
	panic("not implemented")
}
func (c *mockObjectStatusClient) DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error {
	panic("not implemented")
}

func TestGetOrNew(t *testing.T) {
	t.Run("object found", func(t *testing.T) {
		getter := &mockGetter{
			obj: &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
		}
		obj, err := GetOrNew[mockObject, *mockObject]("foo", getter)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, "foo", obj.Name)
	})

	t.Run("object not found", func(t *testing.T) {
		getter := &mockGetter{
			err: apierrors.NewNotFound(schema.GroupResource{Group: "v1", Resource: "pods"}, "foo"),
		}
		obj, err := GetOrNew[mockObject, *mockObject]("foo", getter)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, "foo", obj.Name)
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("some error")
		getter := &mockGetter{
			err: expectedErr,
		}
		_, err := GetOrNew[mockObject, *mockObject]("foo", getter)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})
}

func TestCreateOrUpdate(t *testing.T) {
	t.Run("create new object", func(t *testing.T) {
		getter := &mockGetter{}
		setter := &mockSetter{}
		build := func(o *mockObject) error {
			o.Labels = map[string]string{"key": "value"}
			return nil
		}
		obj, err := CreateOrUpdate[mockObject, *mockObject](context.Background(), "foo", getter, setter, build)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, "foo", obj.Name)
		assert.Equal(t, map[string]string{"key": "value"}, obj.Labels)
		assert.NotNil(t, setter.created)
		assert.Nil(t, setter.updated)
	})

	t.Run("update existing object", func(t *testing.T) {
		getter := &mockGetter{
			obj: &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}},
		}
		setter := &mockSetter{}
		build := func(o *mockObject) error {
			o.Labels = map[string]string{"key": "value"}
			return nil
		}
		obj, err := CreateOrUpdate[mockObject, *mockObject](context.Background(), "foo", getter, setter, build)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, "foo", obj.Name)
		assert.Equal(t, map[string]string{"key": "value"}, obj.Labels)
		assert.Nil(t, setter.created)
		assert.NotNil(t, setter.updated)
	})

	t.Run("no update needed", func(t *testing.T) {
		getter := &mockGetter{
			obj: &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}},
		}
		setter := &mockSetter{}
		build := func(o *mockObject) error {
			return nil
		}
		obj, err := CreateOrUpdate[mockObject, *mockObject](context.Background(), "foo", getter, setter, build)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Nil(t, setter.created)
		assert.Nil(t, setter.updated)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("update object", func(t *testing.T) {
		obj := &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}}
		setter := &mockUpdateClient{}
		build := func(o *mockObject) error {
			o.Labels = map[string]string{"key": "value"}
			return nil
		}
		updatedObj, err := Update(context.Background(), obj, setter, build)
		assert.NoError(t, err)
		assert.NotNil(t, updatedObj)
		assert.Equal(t, map[string]string{"key": "value"}, updatedObj.Labels)
		assert.NotNil(t, setter.updated)
	})

	t.Run("no update needed", func(t *testing.T) {
		obj := &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}}
		setter := &mockUpdateClient{}
		build := func(o *mockObject) error {
			return nil
		}
		updatedObj, err := Update(context.Background(), obj, setter, build)
		assert.NoError(t, err)
		assert.NotNil(t, updatedObj)
		assert.Nil(t, setter.updated)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status", func(t *testing.T) {
		obj := &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}}
		setter := &mockObjectStatusClient{}
		build := func(o *mockObject) error {
			o.Labels = map[string]string{"status": "updated"}
			return nil
		}
		err := UpdateStatus(context.Background(), obj, setter, build, nil)
		assert.NoError(t, err)
		assert.NotNil(t, setter.updated)
		assert.Equal(t, map[string]string{"status": "updated"}, setter.updated.Labels)
	})

	t.Run("no status update needed", func(t *testing.T) {
		obj := &mockObject{ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "1"}}
		setter := &mockObjectStatusClient{}
		build := func(o *mockObject) error {
			return nil
		}
		err := UpdateStatus(context.Background(), obj, setter, build, nil)
		assert.NoError(t, err)
		assert.Nil(t, setter.updated)
	})
}

func TestCleanup(t *testing.T) {
	deleter := &mockDeleter{}
	actual := []*mockObject{
		{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
	}
	expected := []*mockObject{
		{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
	}
	err := Cleanup(context.Background(), actual, expected, deleter)
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, deleter.deleted)
}
