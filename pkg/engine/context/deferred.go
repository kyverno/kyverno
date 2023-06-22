package context

import (
	"regexp"
)

type deferredLoader struct {
	name    string
	matcher regexp.Regexp
	loader  Loader
}

func NewDeferredLoader(name string, loader Loader) (DeferredLoader, error) {
	// match on ASCII word boundaries except do not allow starting with a `.`
	// this allows `x` to match `x.y` but not `y.x` or `y.x.z`
	matcher, err := regexp.Compile(`(?:\A|\z|\s|[^.0-9A-Za-z])` + name + `\b`)
	if err != nil {
		return nil, err
	}

	return &deferredLoader{
		name:    name,
		matcher: *matcher,
		loader:  loader,
	}, nil
}

func (dl *deferredLoader) Name() string {
	return dl.name
}

func (dl *deferredLoader) HasLoaded() bool {
	return dl.loader.HasLoaded()
}

func (dl *deferredLoader) LoadData() error {
	return dl.loader.LoadData()
}

func (d *deferredLoader) Matches(query string) bool {
	return d.matcher.MatchString(query)
}

type byLevel struct {
	level  int
	loader DeferredLoader
}

type deferredLoaders struct {
	enableDeferredLoading bool
	loaders               []byLevel
}

func NewDeferredLoaders(enableDeferredLoading bool) DeferredLoaders {
	return &deferredLoaders{
		enableDeferredLoading: enableDeferredLoading,
		loaders:               make([]byLevel, 0),
	}
}

func (d *deferredLoaders) Enabled() bool {
	return d.enableDeferredLoading
}

func (d *deferredLoaders) Add(dl DeferredLoader, level int) {
	d.loaders = append(d.loaders, byLevel{level, dl})
}

func (d *deferredLoaders) Reset(remove bool, level int) {
	for i := len(d.loaders) - 1; i >= 0; i-- {
		if d.loaders[i].level > level {
			// remove loaders from a nested context (level > current)
			d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
		} else {
			if d.loaders[i].loader.HasLoaded() {
				// reload data into the current context
				if err := d.loaders[i].loader.LoadData(); err != nil {
					logger.Error(err, "failed to reload context entry", "name", d.loaders[i].loader.Name())
				}
				if d.loaders[i].level == level {
					d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
				}
			}
		}
	}
}

func (d *deferredLoaders) Match(query string, level int) DeferredLoader {
	for i, dl := range d.loaders {
		if dl.loader.HasLoaded() {
			continue
		}

		if dl.loader.Matches(query) {
			if dl.level == level {
				d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
			}

			return dl.loader
		}
	}

	return nil
}
