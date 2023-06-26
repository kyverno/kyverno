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

type leveledLoader struct {
	level   int
	matched bool
	loader  DeferredLoader
}

func (cl *leveledLoader) Level() int {
	return cl.level
}

func (cl *leveledLoader) Name() string {
	return cl.loader.Name()
}

func (cl *leveledLoader) Matches(query string) bool {
	return cl.loader.Matches(query)
}

func (cl *leveledLoader) HasLoaded() bool {
	return cl.loader.HasLoaded()
}

func (cl *leveledLoader) LoadData() error {
	return cl.loader.LoadData()
}

type deferredLoaders struct {
	enableDeferredLoading bool
	level                 *int
	loaders               []*leveledLoader
}

func NewDeferredLoaders(enableDeferredLoading bool) DeferredLoaders {
	return &deferredLoaders{
		enableDeferredLoading: enableDeferredLoading,
		loaders:               make([]*leveledLoader, 0),
	}
}

func (d *deferredLoaders) Enabled() bool {
	return d.enableDeferredLoading
}

func (d *deferredLoaders) Add(dl DeferredLoader, level int) {
	d.loaders = append(d.loaders, &leveledLoader{level, false, dl})
}

func (d *deferredLoaders) Reset(restore bool, level int) {
	for i := len(d.loaders) - 1; i >= 0; i-- {
		d.loaders[i].matched = false
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
			} else if !restore {
				if d.loaders[i].level == level {
					d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
				}
			}
		}
	}
}

func (d *deferredLoaders) LoadMatching(query string, level int) error {
	if d.level != nil {
		level = *d.level
	}

	for loader := d.match(query, level); loader != nil; loader = d.match(query, level) {
		l := loader.Level()
		d.level = &l

		if err := loader.LoadData(); err != nil {
			return err
		}

		d.level = nil
	}

	return nil
}

func (d *deferredLoaders) match(query string, level int) LeveledLoader {
	for i, dl := range d.loaders {
		if dl.matched || dl.loader.HasLoaded() {
			continue
		}

		if dl.Matches(query) && dl.level <= level {
			if dl.level == level {
				// remove loaders at current level after execution
				d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
			} else {
				d.loaders[i].matched = true
			}

			return dl
		}
	}

	return nil
}
