package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"k8s.io/client-go/kubernetes"
)

func lookupImports(in reflect.Type) map[string]string {
	imports := map[string]string{}
	for i := 0; i < in.NumMethod(); i++ {
		clientType := in.Method(i).Type.Out(0)
		imports["client_"+strings.ToLower(clientType.Name())] = clientType.PkgPath()
		for j := 0; j < clientType.NumMethod(); j++ {
			resourceType := clientType.Method(j).Type.Out(0)
			imports["client_"+strings.ToLower(clientType.Name())+"_"+strings.ToLower(resourceType.Name())] = resourceType.PkgPath()
			for k := 0; k < resourceType.NumMethod(); k++ {
				method := resourceType.Method(k)
				prefix := "api_" + strings.ToLower(clientType.Name()) + "_" + strings.ToLower(resourceType.Name()) + "_" + strings.ToLower(method.Name)
				for a := 0; a < method.Type.NumIn(); a++ {
					arg := method.Type.In(a)
					if arg.Kind() == reflect.Pointer {
						arg = arg.Elem()
					}
					p := arg.PkgPath()
					if p != "" {
						imports[prefix+"_in_"+strconv.Itoa(a)] = p
					}
				}
				for a := 0; a < method.Type.NumOut(); a++ {
					arg := method.Type.Out(a)
					if arg.Kind() == reflect.Pointer {
						arg = arg.Elem()
					}
					p := arg.PkgPath()
					if p != "" {
						imports[prefix+"_out_"+strconv.Itoa(a)] = p
					}
				}
			}
		}
	}
	imports2 := map[string]string{}
	imports2["context"] = "context"
	imports2["metav1"] = "k8s.io/apimachinery/pkg/apis/meta/v1"
	imports2["types"] = "k8s.io/apimachinery/pkg/types"
	imports2["restclient"] = "k8s.io/client-go/rest"
	imports2["watch"] = "k8s.io/apimachinery/pkg/watch"
	imports2["metrics"] = "github.com/kyverno/kyverno/pkg/metrics"
	for _, v := range imports {
		if v != "" {
			k := strings.ReplaceAll(v, ".", "_")
			k = strings.ReplaceAll(k, "-", "_")
			k = strings.ReplaceAll(k, "/", "_")
			imports2[k] = v
		}
	}
	for k, v := range imports2 {
		fmt.Println(k + " -> " + v)
	}
	return imports2
}

func lookupImport(in string, imports map[string]string) string {
	for k, v := range imports {
		if v == in {
			return k
		}
	}
	return ""
}

func resolveType(in reflect.Type, imports map[string]string) string {
	switch in.Kind() {
	case reflect.Pointer:
		return "*" + resolveType(in.Elem(), imports)
	case reflect.Array:
		return "[]" + resolveType(in.Elem(), imports)
	case reflect.Slice:
		return "[]" + resolveType(in.Elem(), imports)
	case reflect.Map:
		return "map[" + resolveType(in.Key(), imports) + "]" + resolveType(in.Elem(), imports)
	}
	pack := lookupImport(in.PkgPath(), imports)
	if pack == "" {
		return in.Name()
	}
	return pack + "." + in.Name()
}

func main() {
	log.Println("loading packages...")
	t := reflect.TypeOf((*kubernetes.Interface)(nil)).Elem()
	log.Println(t)
	for i := 0; i < t.NumMethod(); i++ {
		log.Println(t.Method(i))
	}
	imports := lookupImports(t)
	tpl := `
package kubernetes

import (
	versioned {{ Quote .PkgPath }}
	{{- range $alias, $package := Imports }}
	{{ $alias }} {{ Quote $package }}
	{{- end }}
)

type clientset struct {
	inner versioned.Interface
	{{- range $cmethod := Methods . }}
	{{- $clientType := index (Out $cmethod) 0 }}
	{{ ToLower $cmethod.Name }} {{ Type $clientType }}
	{{- end }}
}

func Wrap(inner versioned.Interface, m metrics.MetricsConfigManager) versioned.Interface {
	return &clientset{
		inner: inner,
		{{- range $cmethod := Methods . }}
		{{- $clientType := index (Out $cmethod) 0 }}
		{{ ToLower $cmethod.Name }}: wrap{{ $clientType.Name }}(inner.{{ $cmethod.Name }}(), m),
		{{- end }}
	}
}

{{- range $cmethod := Methods . }}
{{- $clientType := index (Out $cmethod) 0 }}
func (c *clientset) {{ $cmethod.Name }}() {{ Type $clientType }} {
	return c.{{ ToLower $cmethod.Name }}
}
{{- end }}

{{- range $cmethod := Methods . }}
{{- $clientType := index (Out $cmethod) 0 }}
type wrapped{{ $clientType.Name }} struct {
	inner   {{ Type $clientType }}
	metrics metrics.MetricsConfigManager
}

func wrap{{ $clientType.Name }}(inner {{ Type $clientType }}, metrics metrics.MetricsConfigManager) {{ Type $clientType }} {
	return &wrapped{{ $clientType.Name }}{inner, metrics}
}

{{- range $rmethod := Methods $clientType }}
{{- if ne $rmethod.Name "RESTClient" }}
{{- $resourceType := index (Out $rmethod) 0 }}
type wrapped{{ $clientType.Name }}{{ $resourceType.Name }} struct {
	inner    {{ Type $resourceType }}
	recorder metrics.Recorder
}

func wrap{{ $clientType.Name }}{{ $resourceType.Name }}(inner {{ Type $resourceType }}, recorder metrics.Recorder) {{ Type $resourceType }} {
	return &wrapped{{ $clientType.Name }}{{ $resourceType.Name }}{inner, recorder}
}

{{- range $emethod := Methods $resourceType }}
func (c *wrapped{{ $clientType.Name }}{{ $resourceType.Name }}) {{ $emethod.Name }}(
	{{- range $i, $argType := In $emethod -}}
	{{- if IsVariadic $emethod $i -}}
	arg{{ $i }} ...{{ Type $argType.Elem }},
	{{- else -}}
	arg{{ $i }} {{ Type $argType }},
	{{- end -}}
	{{- end -}}
) (
	{{- range $returnType := Out $emethod -}}
	{{ Type $returnType }},
	{{- end -}}
) {
	defer c.recorder.Record({{ Quote $emethod.Name }})
	return c.inner.{{ $emethod.Name }}(
		{{- range $i, $_ := In $emethod -}}
		{{- if IsVariadic $emethod $i -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
}
{{- end }}

func (c *wrapped{{ $clientType.Name }}) {{ $rmethod.Name }}(
	{{- range $i, $argType := In $rmethod -}}
	arg{{ $i }} {{ Type $argType }},
	{{- end -}}
) (
	{{- range $returnType := Out $rmethod -}}
	{{ Type $returnType }},
	{{- end -}}
) {
	{{- $returnType := index (Out $rmethod) 0 }}
	{{- if IsNamespaced $rmethod }}
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, {{ Quote (Kind $returnType.Name) }}, metrics.KubeClient)
	{{- else }}
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, {{ Quote (Kind $returnType.Name) }}, metrics.KubeClient)
	{{- end }}
	return wrap{{ $clientType.Name }}{{ $resourceType.Name }}(c.inner.{{ $rmethod.Name }}(
		{{- range $i, $_ := In $rmethod -}}
		arg{{ $i }},
		{{- end -}}
	), recorder)
}
{{- end }}
{{- end }}
func (c *wrapped{{ $clientType.Name }}) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}
{{- end }}
`
	tmpl := template.New("xxx")
	tmpl.Funcs(
		template.FuncMap{
			"Imports": func() map[string]string {
				return imports
			},
			"Import": func(t reflect.Type) string {
				pkg := t.PkgPath()
				for k, v := range imports {
					if v == pkg {
						return k
					}
				}
				return ""
			},
			"Methods": func(t reflect.Type) []reflect.Method {
				var methods []reflect.Method
				for i := 0; i < t.NumMethod(); i++ {
					if t.Method(i).Name != "Discovery" {
						methods = append(methods, t.Method(i))
					}
				}
				return methods
			},
			"PkgPath": func(t reflect.Type) string {
				return t.String()
			},
			"Out": func(in reflect.Method) []reflect.Type {
				var out []reflect.Type
				for i := 0; i < in.Type.NumOut(); i++ {
					out = append(out, in.Type.Out(i))
				}
				return out
			},
			"In": func(in reflect.Method) []reflect.Type {
				var out []reflect.Type
				for i := 0; i < in.Type.NumIn(); i++ {
					out = append(out, in.Type.In(i))
				}
				return out
			},
			"ToLower": func(in string) string {
				return strings.ToLower(in)
			},
			"Quote": func(in string) string {
				return `"` + in + `"`
			},
			"Type": func(in reflect.Type) string {
				return resolveType(in, imports)
			},
			"IsVariadic": func(in reflect.Method, idx int) bool {
				return idx == in.Type.NumIn()-1 && in.Type.IsVariadic()
			},
			"Kind": func(in string) string {
				return strings.ReplaceAll(in, "Interface", "")
			},
			"IsNamespaced": func(in reflect.Method) bool {
				return in.Type.NumIn() != 0
			},
		},
	)
	if tmpl, err := tmpl.Parse(tpl); err != nil {
		panic(err)
	} else {
		folder := "pkg/clients/wrappers/kube"
		if err := os.MkdirAll(folder, 0o755); err != nil {
			panic(fmt.Sprintf("Failed to create directories for %s", folder))
		}
		file := "clientset.generated.go"
		f, err := os.Create(path.Join(folder, file))
		if err != nil {
			panic(fmt.Sprintf("Failed to create file %s", path.Join(folder, file)))
		}
		if err := tmpl.Execute(f, t); err != nil {
			panic(err)
		}
	}
}
