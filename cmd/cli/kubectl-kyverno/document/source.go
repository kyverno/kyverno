package document

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
)

type Predicate = func(fs.FileInfo) bool

type Source interface {
	GetDocuments(Predicate) ([]Document, error)
}

func defaultPredicate(fs.FileInfo) bool {
	return true
}

func NewSource(src string) (Source, error) {
	if IsGit(src) {
		gitURL, err := url.Parse(src)
		if err != nil {
			return nil, err
		}
		pathElems := strings.Split(gitURL.Path[1:], "/")
		if len(pathElems) <= 1 {
			return nil, fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch (without --git-branch flag) OR https://github.com/:owner/:repository/:directory (with --git-branch flag)", gitURL.Path)
		}
		gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
		repoURL := gitURL.String()
		// var gitPathToYamls string
		gitBranch := "main"
		// if gitBranch == "" {
		// 	gitPathToYamls = "/"
		// 	if string(path[len(path)-1]) == "/" {
		// 		gitBranch = strings.ReplaceAll(path, repoURL+"/", "")
		// 	} else {
		// 		gitBranch = strings.ReplaceAll(path, repoURL, "")
		// 	}
		// 	if gitBranch == "" {
		// 		gitBranch = "main"
		// 	} else if string(gitBranch[0]) == "/" {
		// 		gitBranch = gitBranch[1:]
		// 	}
		// } else {
		// 	if string(path[len(path)-1]) == "/" {
		// 		gitPathToYamls = strings.ReplaceAll(path, repoURL+"/", "/")
		// 	} else {
		// 		gitPathToYamls = strings.ReplaceAll(path, repoURL, "/")
		// 	}
		// }
		fs := memfs.New()
		if _, err := gitutils.Clone(repoURL, fs, gitBranch); err != nil {
			return nil, fmt.Errorf("Error: failed to clone repository \nCause: %s\n", err)
		}
		return billyFileSystem{fs}, nil
	} else if IsHttp(src) {
		return nil, nil
	} else {
		return fileSystem(src), nil
	}
}

type billyFileSystem struct {
	billy.Filesystem
}

func (s billyFileSystem) GetDocuments(predicate Predicate) ([]Document, error) {
	if predicate == nil {
		predicate = defaultPredicate
	}
	return getDocuments2(s, ".", predicate)
}

func getDocuments2(fs billy.Filesystem, path string, predicate Predicate) ([]Document, error) {
	if _, err := fs.Stat(path); err != nil {
		return nil, err
	}
	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var docs []Document
	for _, file := range files {
		name := filepath.Join(path, file.Name())
		if file.IsDir() {
			children, err := getDocuments2(fs, name, predicate)
			if err != nil {
				return nil, err
			}
			docs = append(docs, children...)
		} else if predicate(file) {
			docs = append(docs, billyDocument{fs, filepath.Join(path, file.Name())})
		}
	}
	return docs, nil
}

type fileSystem string

func (s fileSystem) GetDocuments(predicate Predicate) ([]Document, error) {
	if predicate == nil {
		predicate = defaultPredicate
	}
	return getDocuments(string(s), predicate)
	// for _, file := range files {
	// 	if file.IsDir() {
	// 		ps, err := loadLocalTest(filepath.Join(path, file.Name()), fileName)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		tests = append(tests, ps...)
	// 	} else if file.Name() == fileName {
	// 		tests = append(tests, LoadTest(nil, filepath.Join(path, fileName)))
	// 	}
	// }
}

func getDocuments(path string, predicate Predicate) ([]Document, error) {
	var docs []Document
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			inner, err := getDocuments(filepath.Join(path, file.Name()), predicate)
			if err != nil {
				return nil, err
			}
			docs = append(docs, inner...)
		} else if info, err := file.Info(); err == nil && predicate(info) {
			docs = append(docs, fileDocument(filepath.Join(path, file.Name())))
		}
	}
	return docs, nil
}
