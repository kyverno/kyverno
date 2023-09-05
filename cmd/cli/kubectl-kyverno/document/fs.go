package document

import (
	"errors"
	"os"
)

type fileDocument string

func (d fileDocument) Content() ([]byte, error) {
	path := string(d)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("document is not file")
	}
	return os.ReadFile(path)
}
