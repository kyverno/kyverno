package context

import (
	"regexp"

	"github.com/go-logr/logr"
)

type deferredLoader struct {
	name    string
	matcher regexp.Regexp
	loader  Loader
	logger  logr.Logger
}

func NewDeferredLoader(name string, loader Loader, logger logr.Logger) (DeferredLoader, error) {
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
		logger:  logger,
	}, nil
}

func (dl *deferredLoader) Name() string {
	return dl.name
}

func (dl *deferredLoader) HasLoaded() bool {
	return dl.loader.HasLoaded()
}

func (dl *deferredLoader) LoadData() error {
	if err := dl.loader.LoadData(); err != nil {
		dl.logger.Error(err, "failed to load data", "name", dl.name)
		return err
	}
	return nil
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
	level   int
	index   int
	loaders []*leveledLoader
}

func NewDeferredLoaders() DeferredLoaders {
	return &deferredLoaders{
		loaders: make([]*leveledLoader, 0),
		level:   -1,
		index:   -1,
	}
}

func (d *deferredLoaders) Add(dl DeferredLoader, level int) {
	d.loaders = append(d.loaders, &leveledLoader{level, false, dl})
}

func (d *deferredLoaders) Reset(restore bool, level int) {
	d.clearMatches()
	for i := 0; i < len(d.loaders); i++ {
		l := d.loaders[i]
		if l.level > level {
			i = d.removeLoader(i)
		} else {
			if l.loader.HasLoaded() {
				// reload data into the current context for restore, and
				// for a reset but only if loader is at a prior level
				if restore || (l.level < level) {
					if err := d.loadData(l, i); err != nil {
						logger.Error(err, "failed to reload context entry", "name", l.loader.Name())
					}
				}
				if l.level == level {
					i = d.removeLoader(i)
				}
			} else if !restore {
				if l.level == level {
					i = d.removeLoader(i)
				}
			}
		}
	}
}

// removeLoader removes loader at the specified index
// and returns the prior index
func (d *deferredLoaders) removeLoader(i int) int {
	d.loaders = append(d.loaders[:i], d.loaders[i+1:]...)
	return i - 1
}

func (d *deferredLoaders) clearMatches() {
	for _, dl := range d.loaders {
		dl.matched = false
	}
}

func (d *deferredLoaders) LoadMatching(query string, level int) error {
	if d.level >= 0 {
		level = d.level
	}

	index := len(d.loaders)
	if d.index >= 0 {
		index = d.index
	}

	for l, idx := d.match(query, level, index); l != nil; l, idx = d.match(query, level, index) {
		if err := d.loadData(l, idx); err != nil {
			return nil
		}
	}

	return nil
}

func (d *deferredLoaders) loadData(l *leveledLoader, index int) error {
	d.setLevelAndIndex(l.level, index)
	defer d.setLevelAndIndex(-1, -1)
	if err := l.LoadData(); err != nil {
		return err
	}

	return nil
}

func (d *deferredLoaders) setLevelAndIndex(level, index int) {
	d.level = level
	d.index = index
}

func (d *deferredLoaders) match(query string, level, index int) (*leveledLoader, int) {
	for i := 0; i < index; i++ {
		dl := d.loaders[i]
		if dl.matched || dl.loader.HasLoaded() {
			continue
		}

		if dl.Matches(query) && dl.level <= level {
			idx := i
			d.loaders[i].matched = true
			return dl, idx
		}
	}

	return nil, -1
}
