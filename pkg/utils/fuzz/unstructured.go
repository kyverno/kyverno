package fuzz

import (
	"strings"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Creates an unstructured k8s object
func CreateUnstructuredObject(f *fuzz.ConsumeFuzzer, typeToCreate string) (*unstructured.Unstructured, error) {
	labels, err := createLabels(f)
	if err != nil {
		return nil, err
	}

	versionAndKind, err := getVersionAndKind(f, typeToCreate)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder

	sb.WriteString("{ ")
	sb.WriteString(versionAndKind)
	sb.WriteString(", \"metadata\": { \"creationTimestamp\": \"2020-09-21T12:56:35Z\", \"name\": \"fuzz\", \"labels\": { ")
	sb.WriteString(labels)
	sb.WriteString(" } }, \"spec\": { ")

	for i := 0; i < 1000; i++ {
		typeToAdd, err := f.GetInt()
		if err != nil {
			return kubeutils.BytesToUnstructured([]byte(sb.String()))
		}
		switch typeToAdd % 11 {
		case 0:
			sb.WriteString("\"")
		case 1:
			s, err := f.GetString()
			if err != nil {
				return kubeutils.BytesToUnstructured([]byte(sb.String()))
			}
			sb.WriteString(s)
		case 2:
			sb.WriteString("{")
		case 3:
			sb.WriteString("}")
		case 4:
			sb.WriteString("[")
		case 5:
			sb.WriteString("]")
		case 6:
			sb.WriteString(":")
		case 7:
			sb.WriteString(",")
		case 8:
			sb.WriteString(" ")
		case 9:
			sb.WriteString("\t")
		case 10:
			sb.WriteString("\n")
		}
	}
	return kubeutils.BytesToUnstructured([]byte(sb.String()))
}

func getVersionAndKind(ff *fuzz.ConsumeFuzzer, typeToCreate string) (string, error) {
	var k, v string
	if typeToCreate == "" {
		kindToCreate, err := ff.GetInt()
		if err != nil {
			return "", err
		}
		k = k8sKinds[kindToCreate%len(k8sKinds)]
	} else {
		k = typeToCreate
		if _, ok := kindToVersion[k]; !ok {
			panic("Type not found")
		}
	}

	v = kindToVersion[k]

	var sb strings.Builder
	sb.WriteString("\"apiVersion\": \"")
	sb.WriteString(v)
	sb.WriteString("\", \"kind\": \"")
	sb.WriteString(k)
	sb.WriteString("\"")
	return sb.String(), nil
}

func createLabels(ff *fuzz.ConsumeFuzzer) (string, error) {
	var sb strings.Builder
	noOfLabels, err := ff.GetInt()
	if err != nil {
		return "", err
	}
	for i := 0; i < noOfLabels%30; i++ {
		key, err := GetK8sString(ff)
		if err != nil {
			return "", err
		}
		value, err := GetK8sString(ff)
		if err != nil {
			return "", err
		}
		sb.WriteString("\"")
		sb.WriteString(key)
		sb.WriteString("\":")
		sb.WriteString("\"")
		sb.WriteString(value)
		sb.WriteString("\"")
		if i != (noOfLabels%30)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String(), nil
}
