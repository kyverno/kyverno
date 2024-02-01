package invalid

import (
	"github.com/pkg/errors"
)

type invalidentry struct {
	err error
}

func (i *invalidentry) Get() (interface{}, error) {
	return nil, errors.Wrapf(i.err, "failed to create cached context entry")
}

func (i *invalidentry) Stop() {}

func New(err error) *invalidentry {
	return &invalidentry{
		err: err,
	}
}
