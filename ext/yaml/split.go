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
		if err != nil {
			if err == io.EOF {
				break
			}
			return documents, fmt.Errorf("unable to read yaml")
		}
		if len(b) == 0 {
			continue
		}
		if !IsEmptyDocument(b) {
			documents = append(documents, b)
		}
	}
	return documents, nil
}
