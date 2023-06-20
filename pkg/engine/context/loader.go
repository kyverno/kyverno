package context

// Loader fetches or produces data and loads it into the context. A loader is created for each
// context entry (e.g. `context.variable`, `context.apiCall`, etc.)
// Loaders are invoked lazily based on variable lookups. Loaders may be invoked multipe times to
// handle checkpoints and restores that occur when processing loops. A loader that fetches remote
// data should be able to handle multiple invocations in an optimal manner by mantaining internal
// state and caching remote data. For example, if an API call is made the data retrieved can be
// stored so that it can be saved in the outer context when a restore is performed.
type Loader interface {
	// Load data fetches or produces data and stores it in the context
	LoadData() error
	// Has loaded indicates if the loader has previously executed
	// and stored data in the context
	HasLoaded() bool
}

// DeferredLoader wraps a Loader and implements context specific behaviors.
// A `level` is used to track the checkpoint level at which the loader was
// created. If the level when loading occurs matches the loader's creation
// level, the loader is discarded after execution. Otherwise, the loader is
// retained so that it can be applied to the prior level when the checkpoint
// is restored or reset.
type DeferredLoader interface {
	Name() string
	Matches(query string) bool
	HasLoaded() bool
	LoadData() error
	GetLevel() int
	SetLevel(level int)
}

// DeferredLoaders manages a list of DeferredLoader instances
type DeferredLoaders interface {
	Enabled() bool
	Add(loader DeferredLoader, level int)
	Match(query string) DeferredLoader
	Checkpoint(level int)
	Reset(remove bool, level int)
}
