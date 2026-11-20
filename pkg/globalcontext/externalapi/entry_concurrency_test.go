package externalapi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/require"
)

// gatedProjectionQuery makes a refresh stop inside projection evaluation
// while policy reads attempt to use the previously published cache snapshot.
type gatedProjectionQuery struct {
	started chan struct{}
	release <-chan struct{}
	value   any
	err     error
}

func (q *gatedProjectionQuery) Search(_ any) (any, error) {
	close(q.started)
	<-q.release
	return q.value, q.err
}

func TestEntryGetDuringProjectionRefresh(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()

	query := &gatedProjectionQuery{
		started: make(chan struct{}),
		release: release,
		value:   "new-result",
	}
	e := &entry{
		dataMap: map[string]any{
			"":       map[string]any{"revision": "old"},
			"result": "old-result",
		},
		projections: []store.Projection{{Name: "result", JP: query}},
	}

	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		e.setData(context.Background(), []byte(`{"revision":"new"}`), nil)
	}()

	select {
	case <-query.started:
	case <-time.After(5 * time.Second):
		t.Fatal("projection refresh did not start")
	}

	type readResult struct {
		value any
		err   error
	}
	for _, tc := range []struct {
		name       string
		projection string
		want       any
	}{
		{name: "raw", projection: "", want: map[string]any{"revision": "old"}},
		{name: "named", projection: "result", want: "old-result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readDone := make(chan readResult, 1)
			go func() {
				v, err := e.Get(tc.projection)
				readDone <- readResult{v, err}
			}()
			select {
			case got := <-readDone:
				require.NoError(t, got.err)
				require.Equal(t, tc.want, got.value)
			case <-time.After(time.Second):
				t.Error("Get blocked behind an in-progress projection while cached data was available")
			}
		})
	}

	unblock()
	select {
	case <-refreshDone:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not complete after projection was released")
	}

	raw, err := e.Get("")
	require.NoError(t, err)
	require.Equal(t, map[string]any{"revision": "new"}, raw)
	projected, err := e.Get("result")
	require.NoError(t, err)
	require.Equal(t, "new-result", projected)
}

func TestEntryProjectionErrorKeepsPreviousSnapshot(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()

	projectionErr := errors.New("projection failed")
	query := &gatedProjectionQuery{
		started: make(chan struct{}),
		release: release,
		err:     projectionErr,
	}
	oldRaw := map[string]any{"revision": "old"}
	e := &entry{
		dataMap:     map[string]any{"": oldRaw, "result": "old-result"},
		projections: []store.Projection{{Name: "result", JP: query}},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		e.setData(context.Background(), []byte(`{"revision":"new"}`), nil)
	}()
	select {
	case <-query.started:
	case <-time.After(5 * time.Second):
		t.Fatal("projection refresh did not start")
	}

	readDone := make(chan error, 1)
	go func() {
		value, err := e.Get("result")
		if err == nil && value != "old-result" {
			err = errors.New("previous cached projection changed during refresh")
		}
		readDone <- err
	}()
	select {
	case err := <-readDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Error("Get blocked during a failing projection")
	}

	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("failing projection refresh did not terminate")
	}

	_, err := e.Get("result")
	require.ErrorIs(t, err, projectionErr)
	e.RLock()
	rawSnapshot := e.dataMap[""]
	projectionSnapshot := e.dataMap["result"]
	e.RUnlock()
	require.Equal(t, oldRaw, rawSnapshot)
	require.Equal(t, "old-result", projectionSnapshot)

	// A subsequent successful refresh must clear the error and publish new data.
	e.projections = nil
	e.setData(context.Background(), []byte(`{"revision":"recovered"}`), nil)
	recovered, err := e.Get("")
	require.NoError(t, err)
	require.Equal(t, map[string]any{"revision": "recovered"}, recovered)
}
