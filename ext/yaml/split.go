package yaml

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// SplitDocuments reads the YAML bytes per-document, unmarshals the TypeMeta information from each document
// and returns a map between the GroupVersionKind of the document and the document bytes
func SplitDocuments(yamlBytes document) ([]document, error) {
	var documents []document
	buf := bytes.NewBuffer(yamlBytes)
	reader := yaml.NewYAMLReader(bufio.NewReader(buf))
	for {
		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		// A real read error (e.g. an invalid "---" document separator) also
		// comes back with an empty b, so it must be checked before the
		// len(b) == 0 case below, or it looks identical to a clean io.EOF
		// and gets silently swallowed instead of returned.
		if err != nil && err != io.EOF {
			return documents, fmt.Errorf("unable to read yaml: %w", err)
		}
		if len(b) == 0 {
			break
		}
		if !IsEmptyDocument(b) {
			documents = append(documents, b)
		}
		if err == io.EOF {
			break
		}
	}
	return documents, nil
}
