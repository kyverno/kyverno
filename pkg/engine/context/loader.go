package context

// Loader fetches or produces data and loads it into the context. A loader is created for each
// context entry (e.g. `context.variable`, `context.apiCall`, etc.)
// Loaders are invoked lazily based on variable lookups. Loaders may be invoked multiple times to
// handle checkpoints and restores that occur when processing loops. A loader that fetches remote
// data should be able to handle multiple invocations in an optimal manner by mantaining internal
// state and caching remote data. For example, if an API call is made the data retrieved can be
// stored so that it can be saved in the outer context when a restore is performed.
type Loader interface {
	// Load data fetches or produces data and stores it in the context
	LoadData() error
	// Has loaded indicates if the loader has previously
	// executed and stored data in a context
	HasLoaded() bool
}
