package invalid

import (
	"github.com/pkg/errors"
)

type entry struct {
	err error
}

func (i *entry) Get() (any, error) {
	return nil, errors.Wrapf(i.err, "failed to create cached context entry")
}

func (i *entry) Stop() {}

func New(err error) *entry {
	return &entry{
		err: err,
	}
}
