package document

import (
	"io"

	"github.com/go-git/go-billy/v5"
)

func IsGit(in string) bool {
	return IsHttp(in)
}

type billyDocument struct {
	billy.Filesystem
	string
}

func (d billyDocument) Content() ([]byte, error) {
	file, err := d.Open(d.string)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}
