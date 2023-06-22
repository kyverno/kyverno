package context

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestDeferredLoaderMatch(t *testing.T) {
	ctx := newContext()
	mockLoader, _ := addDeferred(ctx, "one", "1")
	assert.Equal(t, 0, mockLoader.invocations)

	val, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
	assert.Equal(t, 1, mockLoader.invocations)

	_, _ = ctx.Query("one")
	assert.Equal(t, 1, mockLoader.invocations)

	ctx = newContext()
	addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one<two", "one")

	addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "(one)", "one")

	addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one.two.three", "one")

	addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one-two", "one")

	addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one; two; three", "one")

	addDeferred(ctx, "one1", "11")
	testCheckMatch(t, ctx, "one1; two; three", "one1")
}

func testCheckMatch(t *testing.T, ctx *context, query, name string) {
	loader := ctx.deferred.Match(query, len(ctx.jsonRawCheckpoints))
	assert.Assert(t, loader != nil, "deferred loader %s not resolved for query `%s`", name, query)
	assert.Equal(t, name, loader.Name(), "deferred loader %s name mismatch for query %s", name, query)
}

func TestDeferredLoaderMismatch(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "one", "1")

	_, err := ctx.Query("oneTwoThree")
	assert.ErrorContains(t, err, `Unknown key "oneTwoThree" in path`)

	_, err = ctx.Query("one1")
	assert.ErrorContains(t, err, `Unknown key "one1" in path`)

	_, err = ctx.Query("one_two")
	assert.ErrorContains(t, err, `Unknown key "one_two" in path`)

	_, err = ctx.Query("\"one-two\"")
	assert.ErrorContains(t, err, `Unknown key "one-two" in path`)

	ctx.AddVariable("two.one", "0")
	val, err := ctx.Query("two.one")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)

	val, err = ctx.Query("one.two.three")
	assert.NilError(t, err)
	assert.Equal(t, nil, val)

	val, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
}

func newContext() *context {
	return &context{
		jp:                 jp,
		jsonRaw:            []byte(`{}`),
		jsonRawCheckpoints: make([][]byte, 0),
		deferred:           NewDeferredLoaders(true),
	}
}

type mockLoader struct {
	name         string
	level        int
	value        interface{}
	hasLoaded    bool
	invocations  int
	eventHandler func(event string)
	ctx          *context
}

func (ml *mockLoader) Name() string {
	return ml.name
}

func (ml *mockLoader) SetLevel(level int) {
	ml.level = level
}

func (ml *mockLoader) GetLevel() int {
	return ml.level
}

func (ml *mockLoader) HasLoaded() bool {
	return ml.hasLoaded
}

func (ml *mockLoader) LoadData() error {
	ml.invocations++
	ml.ctx.AddVariable(ml.name, ml.value)
	ml.hasLoaded = true
	if ml.eventHandler != nil {
		event := fmt.Sprintf("%s=%v", ml.name, ml.value)
		ml.eventHandler(event)
	}

	return nil
}

func (ml *mockLoader) setEventHandler(eventHandler func(string)) {
	ml.eventHandler = eventHandler
}

func addDeferred(ctx *context, name string, value interface{}) (*mockLoader, error) {
	loader := &mockLoader{
		name:  name,
		value: value,
		ctx:   ctx,
	}

	d, err := NewDeferredLoader(name, loader)
	if err != nil {
		return loader, err
	}

	ctx.AddDeferredLoader(d)
	return loader, nil
}

func TestDeferredCheckpointRestore(t *testing.T) {
	ctx := newContext()

	ctx.Checkpoint()
	_, _ = addDeferred(ctx, "unused", "unused")
	mock, _ := addDeferred(ctx, "one", "1")
	ctx.Restore()
	assert.Equal(t, 0, mock.invocations)
	assert.Assert(t, ctx.deferred.Match("unused", len(ctx.jsonRawCheckpoints)) == nil)
	assert.Assert(t, ctx.deferred.Match("one", len(ctx.jsonRawCheckpoints)) == nil)

	_, _ = addDeferred(ctx, "one", "1")
	ctx.Checkpoint()
	assert.Assert(t, ctx.deferred.Match("one", len(ctx.jsonRawCheckpoints)) != nil)
	ctx.Restore()
	assert.Assert(t, ctx.deferred.Match("one", len(ctx.jsonRawCheckpoints)) != nil)
	_, _ = ctx.Query("one")
	assert.Assert(t, ctx.deferred.Match("one", len(ctx.jsonRawCheckpoints)) == nil)

	mock, _ = addDeferred(ctx, "one", "1")
	ctx.Checkpoint()
	val, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
	assert.Equal(t, 1, mock.invocations)

	mock2, _ := addDeferred(ctx, "two", "2")
	val, err = ctx.Query("two")
	assert.NilError(t, err)
	assert.Equal(t, "2", val)
	assert.Equal(t, 1, mock2.invocations)

	ctx.Restore()
	val, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
	assert.Equal(t, 2, mock.invocations)

	_, _ = ctx.Query("one")
	assert.Equal(t, 2, mock.invocations)

	_, err = ctx.Query("two")
	assert.ErrorContains(t, err, `Unknown key "two" in path`)

	ctx.Checkpoint()
	val, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
	assert.Equal(t, 2, mock.invocations)

	_, err = ctx.Query("two")
	assert.ErrorContains(t, err, `Unknown key "two" in path`)

	mock3, _ := addDeferred(ctx, "three", "3")
	val, err = ctx.Query("three")
	assert.NilError(t, err)
	assert.Equal(t, "3", val)
	assert.Equal(t, 1, mock3.invocations)

	ctx.Reset()
	val, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)
	assert.Equal(t, 2, mock.invocations)

	_, err = ctx.Query("two")
	assert.ErrorContains(t, err, `Unknown key "two" in path`)

	_, err = ctx.Query("three")
	assert.ErrorContains(t, err, `Unknown key "three" in path`)
}

func TestDeferredForloop(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "value", "0")
	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)

	ctx.Checkpoint()
	for i := 0; i < 5; i++ {
		ctx.Reset()
		expectedVal := fmt.Sprintf("%d", i)
		mock, _ := addDeferred(ctx, "value", expectedVal)
		val, err := ctx.Query("value")
		assert.NilError(t, err)
		assert.Equal(t, expectedVal, val)
		assert.Equal(t, 1, mock.invocations)
	}

	ctx.Restore()
	val, err = ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)
}

func TestDeferredSameName(t *testing.T) {
	ctx := newContext()
	var sequence []string
	hdlr := func(name string) {
		sequence = append(sequence, name)
	}

	mock1, _ := addDeferred(ctx, "value", "0")
	mock1.setEventHandler(hdlr)

	mock2, _ := addDeferred(ctx, "value", "1")
	mock2.setEventHandler(hdlr)

	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "1", val)

	assert.Equal(t, 1, mock1.invocations)
	assert.Equal(t, 1, mock2.invocations)
	assert.Equal(t, 2, len(sequence))
	assert.Equal(t, sequence[0], "value=0")
	assert.Equal(t, sequence[1], "value=1")
}
