package context

import (
	"regexp"
)

type deferredLoader struct {
	name    string
	matcher regexp.Regexp
	loader  Loader
	level   int
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

func (dl *deferredLoader) SetLevel(level int) {
	dl.level = level
}

func (dl *deferredLoader) GetLevel() int {
	return dl.level
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

type deferredLoaders struct {
	enableDeferredLoading bool
	currentLevel          int
	loaders               []DeferredLoader
}

func NewDeferredLoaders(enableDeferredLoading bool) DeferredLoaders {
	return &deferredLoaders{
		enableDeferredLoading: enableDeferredLoading,
		loaders:               make([]DeferredLoader, 0),
	}
}

func (d *deferredLoaders) Enabled() bool {
	return d.enableDeferredLoading
}

func (d *deferredLoaders) Add(dl DeferredLoader, level int) {
	dl.SetLevel(level)
	d.loaders = append(d.loaders, dl)
}

func (d *deferredLoaders) Checkpoint(level int) {
	d.currentLevel = level
}

func (d *deferredLoaders) Reset(remove bool, level int) {
	d.currentLevel = level

	for i, dl := range d.loaders {
		level := dl.GetLevel()
		if level > d.currentLevel {
			// remove if the loader's level is higher than the current
			// level after a checkpoint reset
			d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
		} else if dl.HasLoaded() {
			// reload data into the current context
			if err := dl.LoadData(); err != nil {
				logger.Error(err, "failed to reload context entry", "name", dl.Name())
			} else {
				d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
			}
		}
	}
}

func (d *deferredLoaders) Match(query string) DeferredLoader {
	for i, dl := range d.loaders {
		if dl.HasLoaded() {
			continue
		}

		if dl.Matches(query) {
			if dl.GetLevel() == d.currentLevel {
				d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
			}

			return dl
		}
	}

	return nil
}
