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
	ml, _ := addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one<two", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "(one)", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one.two.three", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one-two", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one; two; three", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one>two", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one", "1")
	testCheckMatch(t, ctx, "one, two, three", "one", "1", ml)

	ml, _ = addDeferred(ctx, "one1", "11")
	testCheckMatch(t, ctx, "one1", "one1", "11", ml)
}

func testCheckMatch(t *testing.T, ctx *context, query, name, value string, ml *mockLoader) {
	var events []string
	hdlr := func(name string) {
		events = append(events, name)
	}

	ml.setEventHandler(hdlr)

	err := ctx.deferred.LoadMatching(query, len(ctx.jsonRawCheckpoints))
	assert.NilError(t, err)
	assert.Equal(t, 1, len(events), "deferred loader %s not executed for query %s", name, query)
	expected := fmt.Sprintf("%s=%s", name, value)
	assert.Equal(t, expected, events[0], "deferred loader %s name mismatch for query %s; received %s", name, query, events[0])
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
		jsonRaw:            make(map[string]interface{}),
		jsonRawCheckpoints: make([]map[string]interface{}, 0),
		deferred:           NewDeferredLoaders(),
	}
}

type mockLoader struct {
	name         string
	level        int
	value        interface{}
	query        string
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

	// simulate a JMESPath evaluation after loading
	if err := ml.executeQuery(); err != nil {
		return err
	}

	ml.hasLoaded = true
	if ml.eventHandler != nil {
		event := fmt.Sprintf("%s=%v", ml.name, ml.value)
		ml.eventHandler(event)
	}

	return nil
}

func (ml *mockLoader) executeQuery() error {
	if ml.query == "" {
		return nil
	}

	results, err := ml.ctx.Query(ml.query)
	if err != nil {
		return err
	}

	return ml.ctx.AddVariable(ml.name, results)
}

func (ml *mockLoader) setEventHandler(eventHandler func(string)) {
	ml.eventHandler = eventHandler
}

func addDeferred(ctx *context, name string, value interface{}) (*mockLoader, error) {
	return addDeferredWithQuery(ctx, name, value, "")
}

func addDeferredWithQuery(ctx *context, name string, value interface{}, query string) (*mockLoader, error) {
	loader := &mockLoader{
		name:  name,
		value: value,
		ctx:   ctx,
		query: query,
	}

	d, err := NewDeferredLoader(name, loader, logger)
	if err != nil {
		return loader, err
	}

	ctx.AddDeferredLoader(d)
	return loader, nil
}

func TestDeferredReset(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "value", "0")

	ctx.Checkpoint()
	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)
	ctx.Reset()

	val, err = ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)
}

func TestDeferredCheckpointRestore(t *testing.T) {
	ctx := newContext()

	ctx.Checkpoint()
	unused, _ := addDeferred(ctx, "unused", "unused")
	mock, _ := addDeferred(ctx, "one", "1")
	ctx.Restore()
	assert.Equal(t, 0, mock.invocations)
	assert.Equal(t, 0, unused.invocations)

	err := ctx.deferred.LoadMatching("unused", len(ctx.jsonRawCheckpoints))
	assert.NilError(t, err)
	_, err = ctx.Query("unused")
	assert.ErrorContains(t, err, "Unknown key \"unused\" in path")

	err = ctx.deferred.LoadMatching("one", len(ctx.jsonRawCheckpoints))
	assert.NilError(t, err)
	_, err = ctx.Query("one")
	assert.ErrorContains(t, err, "Unknown key \"one\" in path")

	_, _ = addDeferred(ctx, "one", "1")
	ctx.Checkpoint()
	one, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", one)

	ctx.Restore()
	_, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", one)

	ctx.Restore()
	_, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "1", one)

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
	addDeferred(ctx, "value", float64(-1))

	ctx.Checkpoint()
	for i := 0; i < 5; i++ {
		val, err := ctx.Query("value")
		assert.NilError(t, err)
		assert.Equal(t, float64(i-1), val)

		ctx.Reset()
		mock, _ := addDeferred(ctx, "value", float64(i))
		val, err = ctx.Query("value")
		assert.NilError(t, err)
		assert.Equal(t, float64(i), val)
		assert.Equal(t, 1, mock.invocations)
	}

	ctx.Restore()
	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, float64(-1), val)
}

func TestDeferredInvalidReset(t *testing.T) {
	ctx := newContext()

	addDeferred(ctx, "value", "0")
	ctx.Reset() // no checkpoint
	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)

	addDeferred(ctx, "value", "0")
	ctx.Restore() // no checkpoint
	val, err = ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)
}

func TestDeferredValidResetRestore(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "value", "0")

	ctx.Checkpoint()
	addDeferred(ctx, "leak", "leak")
	ctx.Reset()

	_, err := ctx.Query("leak")
	assert.ErrorContains(t, err, `Unknown key "leak" in path`)

	addDeferred(ctx, "value", "0")
	ctx.Checkpoint()
	addDeferred(ctx, "leak", "leak")
	ctx.Restore()

	_, err = ctx.Query("leak")
	assert.ErrorContains(t, err, `Unknown key "leak" in path`)
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

func TestDeferredRecursive(t *testing.T) {
	ctx := newContext()
	addDeferredWithQuery(ctx, "value", "0", "value")
	ctx.Checkpoint()
	val, err := ctx.Query("value")
	assert.NilError(t, err)
	assert.Equal(t, "0", val)
}

func TestJMESPathDependency(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "foo", "foo")
	addDeferredWithQuery(ctx, "one", "1", "foo")

	val, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "foo", val)
}

func TestDeferredHiddenEval(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "foo", "foo")

	ctx.Checkpoint()
	addDeferred(ctx, "foo", "bar")

	val, err := ctx.Query("foo")
	assert.NilError(t, err)
	assert.Equal(t, "bar", val)
}

func TestDeferredNotHidden(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "foo", "foo")
	addDeferredWithQuery(ctx, "one", "1", "foo")

	ctx.Checkpoint()
	addDeferred(ctx, "foo", "bar")

	val, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "foo", val)
}

func TestDeferredNotHiddenOrdered(t *testing.T) {
	ctx := newContext()
	addDeferred(ctx, "foo", "foo")
	addDeferredWithQuery(ctx, "one", "1", "foo")
	addDeferred(ctx, "foo", "baz")

	ctx.Checkpoint()
	addDeferred(ctx, "foo", "bar")
	val, err := ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "foo", val)

	val, err = ctx.Query("foo")
	assert.NilError(t, err)
	assert.Equal(t, "bar", val)

	ctx.Restore()

	val, err = ctx.Query("one")
	assert.NilError(t, err)
	assert.Equal(t, "foo", val)

	val, err = ctx.Query("foo")
	assert.NilError(t, err)
	assert.Equal(t, "baz", val)
}
