package jmespath

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"gotest.tools/v3/assert"
)

func TestQueryCacheReuse(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	query := "object.something"

	first, err := jp.Query(query)
	assert.NilError(t, err)
	second, err := jp.Query(query)
	assert.NilError(t, err)
	assert.Assert(t, first == second)
}

func TestQueryCacheDistinct(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	data := map[string]interface{}{
		"a": map[string]interface{}{"value": 1},
		"b": map[string]interface{}{"value": 2},
	}

	first, err := jp.Query("a.value")
	assert.NilError(t, err)
	second, err := jp.Query("b.value")
	assert.NilError(t, err)

	firstResult, err := first.Search(data)
	assert.NilError(t, err)
	secondResult, err := second.Search(data)
	assert.NilError(t, err)

	assert.Equal(t, firstResult, 1)
	assert.Equal(t, secondResult, 2)
}

func TestSearch(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	data := map[string]interface{}{
		"object": map[string]interface{}{"value": 3},
	}

	proxy, err := jp.Query("object.value")
	assert.NilError(t, err)
	queryResult, err := proxy.Search(data)
	assert.NilError(t, err)

	searchResult, err := jp.Search("object.value", data)
	assert.NilError(t, err)

	assert.Equal(t, searchResult, queryResult)
	assert.Equal(t, searchResult, 3)
}

func TestQueryCacheInvalid(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	if _, err := jp.Query("object."); err == nil {
		t.Fatal("expected error for invalid query")
	}
}

func TestQueryCacheSameKeyDifferentData(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	data1 := map[string]interface{}{
		"object": map[string]interface{}{"value": 1},
	}
	data2 := map[string]interface{}{
		"object": map[string]interface{}{"value": 2},
	}

	result1, err := jp.Search("object.value", data1)
	assert.NilError(t, err)
	result2, err := jp.Search("object.value", data2)
	assert.NilError(t, err)

	assert.Equal(t, result1, 1)
	assert.Equal(t, result2, 2)
}

func TestQueryCacheFunctionCall(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	data := map[string]interface{}{
		"object": map[string]interface{}{"value": "a,b,c"},
	}
	query := "split(object.value, ',')"

	first, err := jp.Query(query)
	assert.NilError(t, err)
	second, err := jp.Query(query)
	assert.NilError(t, err)
	assert.Assert(t, first == second)

	firstResult, err := first.Search(data)
	assert.NilError(t, err)
	secondResult, err := second.Search(data)
	assert.NilError(t, err)

	firstSplit, ok := firstResult.([]any)
	assert.Assert(t, ok)
	secondSplit, ok := secondResult.([]any)
	assert.Assert(t, ok)
	assert.Equal(t, firstSplit[0], "a")
	assert.Equal(t, firstSplit[1], "b")
	assert.Equal(t, firstSplit[2], "c")
	assert.Equal(t, secondSplit[0], "a")
	assert.Equal(t, secondSplit[1], "b")
	assert.Equal(t, secondSplit[2], "c")
}

func BenchmarkQueryRepeated(b *testing.B) {
	jp := New(config.NewDefaultConfiguration(false))
	query := "object.value"
	if _, err := jp.Query(query); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := jp.Query(query); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchRepeated(b *testing.B) {
	jp := New(config.NewDefaultConfiguration(false))
	data := map[string]interface{}{
		"object": map[string]interface{}{"value": 1},
	}
	if _, err := jp.Search("object.value", data); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := jp.Search("object.value", data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryDistinct(b *testing.B) {
	jp := New(config.NewDefaultConfiguration(false))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := jp.Query(fmt.Sprintf("object.key_%d", i)); err != nil {
			b.Fatal(err)
		}
	}
}

func TestQueryCacheConcurrent(t *testing.T) {
	jp := New(config.NewDefaultConfiguration(false))

	data := map[string]interface{}{
		"object": map[string]interface{}{"value": 1},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				proxy, err := jp.Query("object.value")
				if err != nil {
					errCh <- err
					return
				}
				if _, err := proxy.Search(data); err != nil {
					errCh <- err
					return
				}
				if _, err := jp.Search("object.value", data); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}
