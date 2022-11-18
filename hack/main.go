package main

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

type arg struct {
	Type       reflect.Type
	IsVariadic bool
}

type operation struct {
	Method reflect.Method
}

func (o operation) HasContext() bool {
	return o.Method.Type.NumIn() > 0 && goType(o.Method.Type.In(0)) == "context.Context"
}

type resource struct {
	Method     reflect.Method
	Type       reflect.Type
	Operations []operation
}

func (r resource) IsNamespaced() bool {
	return r.Method.Type.NumIn() == 1
}

func (r resource) Kind() string {
	return strings.ReplaceAll(r.Type.Name(), "Interface", "")
}

type client struct {
	Method    reflect.Method
	Type      reflect.Type
	Resources []resource
}

type clientset struct {
	Type      reflect.Type
	Clients   []client
	Discovery []operation
}

func getIns(in reflect.Method) []reflect.Type {
	var out []reflect.Type
	for i := 0; i < in.Type.NumIn(); i++ {
		out = append(out, in.Type.In(i))
	}
	return out
}

func getOuts(in reflect.Method) []reflect.Type {
	var out []reflect.Type
	for i := 0; i < in.Type.NumOut(); i++ {
		out = append(out, in.Type.Out(i))
	}
	return out
}

func getMethods(in reflect.Type) []reflect.Method {
	var out []reflect.Method
	for i := 0; i < in.NumMethod(); i++ {
		out = append(out, in.Method(i))
	}
	return out
}

func packageAlias(in string) string {
	alias := in
	alias = strings.ReplaceAll(alias, ".", "_")
	alias = strings.ReplaceAll(alias, "-", "_")
	alias = strings.ReplaceAll(alias, "/", "_")
	return alias
}

func goType(in reflect.Type) string {
	switch in.Kind() {
	case reflect.Pointer:
		return "*" + goType(in.Elem())
	case reflect.Array:
		return "[]" + goType(in.Elem())
	case reflect.Slice:
		return "[]" + goType(in.Elem())
	case reflect.Map:
		return "map[" + goType(in.Key()) + "]" + goType(in.Elem())
	}
	pack := packageAlias(in.PkgPath())
	if pack == "" {
		return in.Name()
	}
	return pack + "." + in.Name()
}

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func parse(in reflect.Type) clientset {
	cs := clientset{
		Type: in,
	}
	for _, clientMethod := range getMethods(in) {
		// client methods return only the client interface type
		if clientMethod.Type.NumOut() == 1 && clientMethod.Name != "Discovery" {
			clientType := clientMethod.Type.Out(0)
			c := client{
				Method: clientMethod,
				Type:   clientType,
			}
			for _, resourceMethod := range getMethods(clientType) {
				// resource methods return only the resosurce interface type
				if resourceMethod.Type.NumOut() == 1 && resourceMethod.Name != "RESTClient" {
					resourceType := resourceMethod.Type.Out(0)
					r := resource{
						Method: resourceMethod,
						Type:   resourceType,
					}
					for _, operationMethod := range getMethods(resourceType) {
						o := operation{
							Method: operationMethod,
						}
						r.Operations = append(r.Operations, o)
					}
					c.Resources = append(c.Resources, r)
				}
			}
			cs.Clients = append(cs.Clients, c)
		} else if clientMethod.Name == "Discovery" {
			clientType := clientMethod.Type.Out(0)
			for _, operationMethod := range getMethods(clientType) {
				if operationMethod.Name != "RESTClient" {
					o := operation{
						Method: operationMethod,
					}
					cs.Discovery = append(cs.Discovery, o)
				}
			}
		}
	}
	return cs
}

func parseImports(cs clientset, packages ...string) []string {
	imports := sets.NewString(packages...).Insert(cs.Type.PkgPath())
	for _, c := range cs.Clients {
		imports.Insert(c.Type.PkgPath())
		for _, r := range c.Resources {
			imports.Insert(r.Type.PkgPath())
			for _, o := range r.Operations {
				for _, i := range getIns(o.Method) {
					if i.Kind() == reflect.Pointer {
						i = i.Elem()
					}
					if i.PkgPath() != "" {
						imports.Insert(i.PkgPath())
					}
				}
				for _, i := range getOuts(o.Method) {
					if i.Kind() == reflect.Pointer {
						i = i.Elem()
					}
					if i.PkgPath() != "" {
						imports.Insert(i.PkgPath())
					}
				}
			}
		}
	}
	for _, o := range cs.Discovery {
		for _, i := range getIns(o.Method) {
			if i.Kind() == reflect.Pointer {
				i = i.Elem()
			}
			if i.PkgPath() != "" {
				imports.Insert(i.PkgPath())
			}
		}
		for _, i := range getOuts(o.Method) {
			if i.Kind() == reflect.Pointer {
				i = i.Elem()
			}
			if i.PkgPath() != "" {
				imports.Insert(i.PkgPath())
			}
		}
	}
	return imports.List()
}

func executeTemplate(tpl string, cs clientset, folder string, packages ...string) {
	tmpl := template.New("xxx")
	tmpl.Funcs(
		template.FuncMap{
			"ToLower": func(in string) string {
				return strings.ToLower(in)
			},
			"Quote": func(in string) string {
				return `"` + in + `"`
			},
			"SnakeCase": func(in string) string {
				return toSnakeCase(in)
			},
			"Args": func(in reflect.Method) []arg {
				var out []arg
				for i, a := range getIns(in) {
					out = append(out, arg{
						Type:       a,
						IsVariadic: in.Type.IsVariadic() && i == in.Type.NumIn()-1,
					})
				}
				return out
			},
			"Returns": func(in reflect.Method) []reflect.Type {
				return getOuts(in)
			},
			"Pkg": func(in string) string {
				return packageAlias(in)
			},
			"GoType": func(in reflect.Type) string {
				return goType(in)
			},
		},
	)
	if tmpl, err := tmpl.Parse(tpl); err != nil {
		panic(err)
	} else {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			panic(fmt.Sprintf("Failed to create directories for %s", folder))
		}
		file := "clientset.generated.go"
		f, err := os.Create(path.Join(folder, file))
		if err != nil {
			panic(fmt.Sprintf("Failed to create file %s", path.Join(folder, file)))
		}
		if err := tmpl.Execute(f, map[string]interface{}{
			"Clientset": cs,
			"Packages":  parseImports(cs, packages...),
		}); err != nil {
			panic(err)
		}
	}
}

func generateMetricsWrapper(cs clientset, folder string, packages ...string) {
	tpl := `
package client
{{- $clientsetPkg := Pkg .Clientset.Type.PkgPath }}
{{- $metricsPkg := Pkg "github.com/kyverno/kyverno/pkg/metrics" }}
{{- $discoveryPkg := Pkg "k8s.io/client-go/discovery" }}
{{- $restPkg := Pkg "k8s.io/client-go/rest" }}

import (
	{{- range $package := .Packages }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
)

// Wrap
func Wrap(inner {{ GoType .Clientset.Type }}, m {{ $metricsPkg }}.MetricsConfigManager, t {{ $metricsPkg }}.ClientType) {{ GoType .Clientset.Type }} {
	return &clientset{
		discovery: newDiscoveryInterface(inner.Discovery(), m, t),
		{{- range $client := .Clientset.Clients }}
		{{ ToLower $client.Method.Name }}: new{{ $client.Method.Name }}(inner.{{ $client.Method.Name }}(), m, t),
		{{- end }}
	}
}

// NewForConfig
func NewForConfig(c *{{ $restPkg }}.Config, m {{ $metricsPkg }}.MetricsConfigManager, t {{ $metricsPkg }}.ClientType) ({{ GoType .Clientset.Type }}, error) {
	inner, err := {{ $clientsetPkg }}.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return Wrap(inner, m, t), nil
}

// clientset wrapper
type clientset struct {
	discovery {{ $discoveryPkg }}.DiscoveryInterface
	{{- range $client := .Clientset.Clients }}
	{{ ToLower $client.Method.Name }} {{ GoType $client.Type }}
	{{- end }}
}
func (c *clientset) Discovery() {{ $discoveryPkg }}.DiscoveryInterface {
	return c.discovery
}
{{- range $client := .Clientset.Clients }}
func (c *clientset) {{ $client.Method.Name }}() {{ GoType $client.Type }} {
	return c.{{ ToLower $client.Method.Name }}
}
{{- end }}

// wrappedDiscoveryInterface
type wrappedDiscoveryInterface struct {
	inner {{ $discoveryPkg }}.DiscoveryInterface
	metrics    {{ $metricsPkg }}.MetricsConfigManager
	clientType {{ $metricsPkg }}.ClientType
}

func newDiscoveryInterface(inner {{ $discoveryPkg }}.DiscoveryInterface, metrics {{ $metricsPkg }}.MetricsConfigManager, t {{ $metricsPkg }}.ClientType) {{ $discoveryPkg }}.DiscoveryInterface {
	return &wrappedDiscoveryInterface{inner, metrics, t}
}
{{- range $operation := .Clientset.Discovery }}
func (c *wrappedDiscoveryInterface) {{ $operation.Method.Name }}(
	{{- range $i, $arg := Args $operation.Method -}}
	{{- if $arg.IsVariadic -}}
	arg{{ $i }} ...{{ GoType $arg.Type.Elem }},
	{{- else -}}
	arg{{ $i }} {{ GoType $arg.Type }},
	{{- end -}}
	{{- end -}}
) (
	{{- range $return := Returns $operation.Method -}}
	{{ GoType $return }},
	{{- end -}}
) {
	c.metrics.RecordClientQueries({{ Quote (SnakeCase $operation.Method.Name) }}, "Discovery", "", "")
	return c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
}
{{- end }}
func (c *wrappedDiscoveryInterface) RESTClient() {{ $restPkg }}.Interface {
	return c.inner.RESTClient()
}

{{- range $client := .Clientset.Clients }}
{{- $clientGoType := GoType $client.Type }}
// wrapped{{ $client.Method.Name }} wrapper
type wrapped{{ $client.Method.Name }} struct {
	inner      {{ $clientGoType }}
	metrics    {{ $metricsPkg }}.MetricsConfigManager
	clientType {{ $metricsPkg }}.ClientType
}
func new{{ $client.Method.Name }}(inner {{ $clientGoType }}, metrics {{ $metricsPkg }}.MetricsConfigManager, t {{ $metricsPkg }}.ClientType) {{ $clientGoType }} {
	return &wrapped{{ $client.Method.Name }}{inner, metrics, t}
}
{{- range $resource := $client.Resources }}
func (c *wrapped{{ $client.Method.Name }}) {{ $resource.Method.Name }}({{- if $resource.IsNamespaced -}}namespace string{{- end -}}) {{ GoType $resource.Type }} {
	{{- if $resource.IsNamespaced }}
	recorder := {{ $metricsPkg }}.NamespacedClientQueryRecorder(c.metrics, namespace, {{ Quote $resource.Kind }}, c.clientType)
	return new{{ $client.Method.Name }}{{ $resource.Method.Name }}(c.inner.{{ $resource.Method.Name }}(namespace), recorder)
	{{- else }}
	recorder := {{ $metricsPkg }}.ClusteredClientQueryRecorder(c.metrics, {{ Quote $resource.Kind }}, c.clientType)
	return new{{ $client.Method.Name }}{{ $resource.Method.Name }}(c.inner.{{ $resource.Method.Name }}(), recorder)
	{{- end }}
}
{{- end }}
func (c *wrapped{{ $client.Method.Name }}) RESTClient() {{ $restPkg }}.Interface {
	return c.inner.RESTClient()
}
{{- end }}

{{- range $client := .Clientset.Clients }}
{{- range $resource := $client.Resources }}
// wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }} wrapper
type wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }} struct {
	inner    {{ GoType $resource.Type }}
	recorder {{ $metricsPkg }}.Recorder
}
func new{{ $client.Method.Name }}{{ $resource.Method.Name }}(inner {{ GoType $resource.Type }}, recorder {{ $metricsPkg }}.Recorder) {{ GoType $resource.Type }} {
	return &wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }}{inner, recorder}
}
{{- range $operation := $resource.Operations }}
func (c *wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }}) {{ $operation.Method.Name }}(
	{{- range $i, $arg := Args $operation.Method -}}
	{{- if $arg.IsVariadic -}}
	arg{{ $i }} ...{{ GoType $arg.Type.Elem }},
	{{- else -}}
	arg{{ $i }} {{ GoType $arg.Type }},
	{{- end -}}
	{{- end -}}
) (
	{{- range $return := Returns $operation.Method -}}
	{{ GoType $return }},
	{{- end -}}
) {
	defer c.recorder.Record({{ Quote (SnakeCase $operation.Method.Name) }})
	return c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
}
{{- end }}
{{- end }}
{{- end }}
`
	executeTemplate(tpl, cs, folder, packages...)
}

func generateTracesWrapper(cs clientset, folder string, packages ...string) {
	tpl := `
package client
{{- $clientsetPkg := Pkg .Clientset.Type.PkgPath }}
{{- $discoveryPkg := Pkg "k8s.io/client-go/discovery" }}
{{- $restPkg := Pkg "k8s.io/client-go/rest" }}
{{- $tracingPkg := Pkg "github.com/kyverno/kyverno/pkg/tracing" }}
{{- $attributePkg := Pkg "go.opentelemetry.io/otel/attribute" }}

import (
	{{- range $package := .Packages }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
)

// Wrap
func Wrap(inner {{ GoType .Clientset.Type }}) {{ GoType .Clientset.Type }} {
	return &clientset{
		discovery: newDiscoveryInterface(inner.Discovery()),
		{{- range $client := .Clientset.Clients }}
		{{ ToLower $client.Method.Name }}: new{{ $client.Method.Name }}(inner.{{ $client.Method.Name }}()),
		{{- end }}
	}
}

// NewForConfig
func NewForConfig(c *{{ $restPkg }}.Config) ({{ GoType .Clientset.Type }}, error) {
	inner, err := {{ $clientsetPkg }}.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return Wrap(inner), nil
}

// clientset wrapper
type clientset struct {
	discovery {{ $discoveryPkg }}.DiscoveryInterface
	{{- range $client := .Clientset.Clients }}
	{{ ToLower $client.Method.Name }} {{ GoType $client.Type }}
	{{- end }}
}
func (c *clientset) Discovery() {{ $discoveryPkg }}.DiscoveryInterface {
	return c.discovery
}
{{- range $client := .Clientset.Clients }}
func (c *clientset) {{ $client.Method.Name }}() {{ GoType $client.Type }} {
	return c.{{ ToLower $client.Method.Name }}
}
{{- end }}

// wrappedDiscoveryInterface
type wrappedDiscoveryInterface struct {
	inner {{ $discoveryPkg }}.DiscoveryInterface
}

func newDiscoveryInterface(inner {{ $discoveryPkg }}.DiscoveryInterface) {{ $discoveryPkg }}.DiscoveryInterface {
	return &wrappedDiscoveryInterface{inner}
}
{{- range $operation := .Clientset.Discovery }}
func (c *wrappedDiscoveryInterface) {{ $operation.Method.Name }}(
	{{- range $i, $arg := Args $operation.Method -}}
	{{- if $arg.IsVariadic -}}
	arg{{ $i }} ...{{ GoType $arg.Type.Elem }},
	{{- else -}}
	arg{{ $i }} {{ GoType $arg.Type }},
	{{- end -}}
	{{- end -}}
) (
	{{- range $return := Returns $operation.Method -}}
	{{ GoType $return }},
	{{- end -}}
) {
	{{- if $operation.HasContext }}
	ctx, span := {{ $tracingPkg }}.StartSpan(
		arg0,
	{{- else }}
	_, span := {{ $tracingPkg }}.StartSpan(
		context.TODO(),
	{{- end }}
		{{ Quote $.Folder }},
		"KUBE Discovery/{{ $operation.Method.Name }}",
		{{ $attributePkg }}.String("client", "Discovery"),
		{{ $attributePkg }}.String("operation", {{ Quote $operation.Method.Name }}),
	)
	defer span.End()
	{{- if $operation.HasContext }}
	arg0 = ctx
	{{- end }}
	return c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
}
{{- end }}
func (c *wrappedDiscoveryInterface) RESTClient() {{ $restPkg }}.Interface {
	return c.inner.RESTClient()
}

{{- range $client := .Clientset.Clients }}
{{- $clientGoType := GoType $client.Type }}
// wrapped{{ $client.Method.Name }} wrapper
type wrapped{{ $client.Method.Name }} struct {
	inner      {{ $clientGoType }}
}
func new{{ $client.Method.Name }}(inner {{ $clientGoType }}) {{ $clientGoType }} {
	return &wrapped{{ $client.Method.Name }}{inner}
}
{{- range $resource := $client.Resources }}
func (c *wrapped{{ $client.Method.Name }}) {{ $resource.Method.Name }}({{- if $resource.IsNamespaced -}}namespace string{{- end -}}) {{ GoType $resource.Type }} {
	{{- if $resource.IsNamespaced }}
	return new{{ $client.Method.Name }}{{ $resource.Method.Name }}(c.inner.{{ $resource.Method.Name }}(namespace))
	{{- else }}
	return new{{ $client.Method.Name }}{{ $resource.Method.Name }}(c.inner.{{ $resource.Method.Name }}())
	{{- end }}
}
{{- end }}
func (c *wrapped{{ $client.Method.Name }}) RESTClient() {{ $restPkg }}.Interface {
	return c.inner.RESTClient()
}
{{- end }}

{{- range $client := .Clientset.Clients }}
{{- range $resource := $client.Resources }}
// wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }} wrapper
type wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }} struct {
	inner    {{ GoType $resource.Type }}
}
func new{{ $client.Method.Name }}{{ $resource.Method.Name }}(inner {{ GoType $resource.Type }}) {{ GoType $resource.Type }} {
	return &wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }}{inner}
}
{{- range $operation := $resource.Operations }}
func (c *wrapped{{ $client.Method.Name }}{{ $resource.Method.Name }}) {{ $operation.Method.Name }}(
	{{- range $i, $arg := Args $operation.Method -}}
	{{- if $arg.IsVariadic -}}
	arg{{ $i }} ...{{ GoType $arg.Type.Elem }},
	{{- else -}}
	arg{{ $i }} {{ GoType $arg.Type }},
	{{- end -}}
	{{- end -}}
) (
	{{- range $return := Returns $operation.Method -}}
	{{ GoType $return }},
	{{- end -}}
) {
	{{- if $operation.HasContext }}
	ctx, span := {{ $tracingPkg }}.StartSpan(
		arg0,
		"{{ $client.Method.Name }}/{{ $resource.Method.Name }}",
		{{ Quote $operation.Method.Name }},
		{{ $attributePkg }}.String("client", {{ Quote $client.Method.Name }}),
		{{ $attributePkg }}.String("resource", {{ Quote $resource.Method.Name }}),
		{{ $attributePkg }}.String("kind", {{ Quote $resource.Kind }}),
		{{ $attributePkg }}.String("operation", {{ Quote $operation.Method.Name }}),
	)
	defer span.End()
	{{- if $operation.HasContext }}
	arg0 = ctx
	{{- end }}
	return c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
}
{{- end }}
{{- end }}
{{- end }}
`
	executeTemplate(tpl, cs, folder, packages...)
}

func main() {
	kube := parse(reflect.TypeOf((*kubernetes.Interface)(nil)).Elem())
	kyverno := parse(reflect.TypeOf((*versioned.Interface)(nil)).Elem())
	generateMetricsWrapper(
		kube,
		"pkg/clients/wrappers/metrics/kube",
		"context",
		"github.com/kyverno/kyverno/pkg/metrics",
		"k8s.io/client-go/discovery",
		"k8s.io/client-go/rest",
	)
	generateMetricsWrapper(
		kyverno,
		"pkg/clients/wrappers/metrics/kyverno",
		"context",
		"github.com/kyverno/kyverno/pkg/metrics",
		"k8s.io/client-go/discovery",
		"k8s.io/client-go/rest",
	)
	generateTracesWrapper(
		kube,
		"pkg/clients/wrappers/traces/kube",
		"context",
		"github.com/kyverno/kyverno/pkg/tracing",
		"go.opentelemetry.io/otel/attribute",
		"k8s.io/client-go/discovery",
		"k8s.io/client-go/rest",
	)
	generateTracesWrapper(
		kyverno,
		"pkg/clients/wrappers/traces/kyverno",
		"context",
		"github.com/kyverno/kyverno/pkg/tracing",
		"go.opentelemetry.io/otel/attribute",
		"k8s.io/client-go/discovery",
		"k8s.io/client-go/rest",
	)
}
