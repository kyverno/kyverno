package document

import (
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
)

type Document interface {
	Content() ([]byte, error)
}

type fileDocument string

func (d fileDocument) Content() ([]byte, error) {
	return os.ReadFile(string(d))
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
