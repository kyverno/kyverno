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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

const (
	resourceTpl = `
package resource

import (
	"fmt"
	"time"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	{{- range $package := Packages .Target.Type }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
)

func WithLogging(inner {{ GoType .Target.Type }}, logger logr.Logger) {{ GoType .Target.Type }} {
	return &withLogging{inner, logger}
}

func WithMetrics(inner {{ GoType .Target.Type }}, recorder metrics.Recorder) {{ GoType .Target.Type }} {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner {{ GoType .Target.Type }}, client, kind string) {{ GoType .Target.Type }} {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  {{ GoType .Target.Type }}
	logger logr.Logger
}

{{- range $operation := .Target.Operations }}
func (c *withLogging) {{ $operation.Method.Name }}(
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
	start := time.Now()
	logger := c.logger.WithValues("operation", {{ Quote $operation.Method.Name }})
	{{ range $i, $ret := Returns $operation.Method }}ret{{ $i }}{{ if not $ret.IsLast -}},{{- end }} {{ end }} := c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
	{{- if $operation.HasError }}
	if err := multierr.Combine(
		{{- range $i, $ret := Returns $operation.Method -}}
		{{- if $ret.IsError -}}
		ret{{ $i }},
		{{- end -}}
		{{- end -}}
	); err != nil {
		logger.Error(err, "{{ $operation.Method.Name }} failed", "duration", time.Since(start))
	} else {
		logger.Info("{{ $operation.Method.Name }} done", "duration", time.Since(start))
	}
	{{- else }}
	logger.Info("{{ $operation.Method.Name }} done", "duration", time.Since(start))
	{{- end }}
	return	{{ range $i, $ret := Returns $operation.Method -}}
	ret{{ $i }}{{ if not $ret.IsLast -}},{{- end }}
	{{- end }}
}
{{- end }}

type withMetrics struct {
	inner    {{ GoType .Target.Type }}
	recorder metrics.Recorder
}

{{- range $operation := .Target.Operations }}
func (c *withMetrics) {{ $operation.Method.Name }}(
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
	defer c.recorder.RecordWithContext(arg0, {{ Quote (SnakeCase $operation.Method.Name) }})
	{{- else }}
	defer c.recorder.Record({{ Quote (SnakeCase $operation.Method.Name) }})
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

type withTracing struct {
	inner    {{ GoType .Target.Type }}
	client   string
	kind     string
}

{{- range $operation := .Target.Operations }}
func (c *withTracing) {{ $operation.Method.Name }}(
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
	{{- if not $operation.HasContext }}
	return c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
	{{- else }}
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, {{ Quote $operation.Method.Name }}),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String({{ Quote $operation.Method.Name }}),
			),
		)
		defer span.End()
	}
	{{ range $i, $ret := Returns $operation.Method }}ret{{ $i }}{{ if not $ret.IsLast -}},{{- end }} {{ end }} := c.inner.{{ $operation.Method.Name }}(
		{{- range $i, $arg := Args $operation.Method -}}
		{{- if $arg.IsVariadic -}}
		arg{{ $i }}...,
		{{- else -}}
		arg{{ $i }},
		{{- end -}}
		{{- end -}}
	)
	if span != nil {
		{{- if $operation.HasError }}
		{{- range $i, $ret := Returns $operation.Method }}
		{{- if $ret.IsError }}
		tracing.SetSpanStatus(span, ret{{ $i }})
		{{- end }}
		{{- end }}
		{{- end }}
	}
	return	{{ range $i, $ret := Returns $operation.Method -}}
	ret{{ $i }}{{ if not $ret.IsLast -}},{{- end }}
	{{- end }}
	{{- end }}
}
{{- end }}
`
	clientTpl = `
package client

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
	{{- range $package := Packages .Target.Type }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
	{{- range $method, $resource := .Target.Resources }}
	{{ ToLower $method.Name }} "github.com/kyverno/kyverno/{{ $.Folder }}/{{ ToLower $method.Name }}"
	{{- end }}
)

func WithMetrics(inner {{ GoType .Target.Type }}, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) {{ GoType .Target.Type }} {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner {{ GoType .Target.Type }}, client string) {{ GoType .Target.Type }} {
	return &withTracing{inner, client}
}

func WithLogging(inner {{ GoType .Target.Type }}, logger logr.Logger) {{ GoType .Target.Type }} {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      {{ GoType .Target }}
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}
func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
{{- range $method, $resource := .Target.Resources }}
func (c *withMetrics) {{ $method.Name }}({{- if $method.IsNamespaced -}}namespace string{{- end -}}) {{ GoType $resource.Type }} {
	{{- if $method.IsNamespaced }}
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, {{ Quote $resource.Kind }}, c.clientType)
	{{- else }}
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, {{ Quote $resource.Kind }}, c.clientType)
	{{- end }}
	return 	{{ ToLower $method.Name }}.WithMetrics(c.inner.{{ $method.Name }}(
		{{- if $method.IsNamespaced -}}namespace{{- end -}}
	), recorder)
}
{{- end }}

type withTracing struct {
	inner  {{ GoType .Target }}
	client string
}
func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
{{- range $method, $resource := .Target.Resources }}
func (c *withTracing) {{ $method.Name }}({{- if $method.IsNamespaced -}}namespace string{{- end -}}) {{ GoType $resource.Type }} {
	return 	{{ ToLower $method.Name }}.WithTracing(c.inner.{{ $method.Name }}(
		{{- if $method.IsNamespaced -}}namespace{{- end -}}), c.client, {{ Quote $resource.Kind -}}
	)
}
{{- end }}

type withLogging struct {
	inner  {{ GoType .Target }}
	logger logr.Logger
}
func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
{{- range $method, $resource := .Target.Resources }}
func (c *withLogging) {{ $method.Name }}({{- if $method.IsNamespaced -}}namespace string{{- end -}}) {{ GoType $resource.Type }} {
	return 	{{ ToLower $method.Name }}.WithLogging(c.inner.{{ $method.Name }}(
		{{- if $method.IsNamespaced -}}namespace{{- end -}}), c.logger.WithValues("resource", {{ Quote $method.Name }})
		{{- if $method.IsNamespaced -}}.WithValues("namespace", namespace){{- end -}}
	)
}
{{- end }}
`
	clientsetTpl = `
package clientset

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	{{- range $package := Packages .Target.Type }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
	{{- range $resourceMethod, $resource := .Target.Resources }}
	{{ ToLower $resourceMethod.Name }} "github.com/kyverno/kyverno/{{ $.Folder }}/{{ ToLower $resourceMethod.Name }}"
	{{- end }}
	{{- range $clientMethod, $client := .Target.Clients }}
	{{ ToLower $clientMethod.Name }} "github.com/kyverno/kyverno/{{ $.Folder }}/{{ ToLower $clientMethod.Name }}"
	{{- end }}
)

type clientset struct {
	{{- range $resourceMethod, $resource := .Target.Resources }}
	{{ ToLower $resourceMethod.Name }} {{ GoType $resource.Type }}
	{{- end }}
	{{- range $clientMethod, $client := .Target.Clients }}
	{{ ToLower $clientMethod.Name }} {{ GoType $client.Type }}
	{{- end }}
}

{{- range $resourceMethod, $resource := .Target.Resources }}
func (c *clientset) {{ $resourceMethod.Name }}() {{ GoType $resource.Type }}{
	return c.{{ ToLower $resourceMethod.Name }}
}
{{- end }}

{{- range $clientMethod, $client := .Target.Clients }}
func (c *clientset) {{ $clientMethod.Name }}() {{ GoType $client.Type }}{
	return c.{{ ToLower $clientMethod.Name }}
}
{{- end }}

func WrapWithMetrics(inner {{ GoType .Target }}, m metrics.MetricsConfigManager, clientType metrics.ClientType) {{ GoType .Target }} {
	return &clientset{
		{{- range $resourceMethod, $resource := .Target.Resources }}
		{{ ToLower $resourceMethod.Name }}: {{ ToLower $resourceMethod.Name }}.WithMetrics(inner.{{ $resourceMethod.Name }}(), metrics.ClusteredClientQueryRecorder(m, {{ Quote $resource.Kind }}, clientType)),
		{{- end }}
		{{- range $clientMethod, $client := .Target.Clients }}
		{{ ToLower $clientMethod.Name }}: {{ ToLower $clientMethod.Name }}.WithMetrics(inner.{{ $clientMethod.Name }}(), m, clientType),
		{{- end }}
	}
}

func WrapWithTracing(inner {{ GoType .Target }}) {{ GoType .Target }} {
	return &clientset{
		{{- range $resourceMethod, $resource := .Target.Resources }}
		{{ ToLower $resourceMethod.Name }}: {{ ToLower $resourceMethod.Name }}.WithTracing(inner.{{ $resourceMethod.Name }}(), {{ Quote $resourceMethod.Name }}, ""),
		{{- end }}
		{{- range $clientMethod, $client := .Target.Clients }}
		{{ ToLower $clientMethod.Name }}: {{ ToLower $clientMethod.Name }}.WithTracing(inner.{{ $clientMethod.Name }}(), {{ Quote $clientMethod.Name }}),
		{{- end }}
	}
}

func WrapWithLogging(inner {{ GoType .Target }}, logger logr.Logger) {{ GoType .Target }} {
	return &clientset{
		{{- range $resourceMethod, $resource := .Target.Resources }}
		{{ ToLower $resourceMethod.Name }}: {{ ToLower $resourceMethod.Name }}.WithLogging(inner.{{ $resourceMethod.Name }}(), logger.WithValues("group", {{ Quote $resourceMethod.Name }})),
		{{- end }}
		{{- range $clientMethod, $client := .Target.Clients }}
		{{ ToLower $clientMethod.Name }}: {{ ToLower $clientMethod.Name }}.WithLogging(inner.{{ $clientMethod.Name }}(), logger.WithValues("group", {{ Quote $clientMethod.Name }})),
		{{- end }}
	}
}
`
	interfaceTpl = `
package clientset

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	{{- range $package := Packages .Target.Type }}
	{{ Pkg $package }} {{ Quote $package }}
	{{- end }}
	{{- range $clientMethod, $client := .Target.Clients }}
	{{ ToLower $clientMethod.Name }} "github.com/kyverno/kyverno/{{ $.Folder }}/{{ ToLower $clientMethod.Name }}"
	{{- end }}
)

type Interface interface {
	{{ GoType .Target.Type }}
	WithMetrics(metrics.MetricsConfigManager, metrics.ClientType) Interface
	WithTracing() Interface
	WithLogging(logr.Logger) Interface
}

func From(inner {{ GoType .Target }}, opts ...NewOption) Interface {
	i := from(inner)
	for _, opt := range opts {
		i = opt(i)
	}
	return i
}

type NewOption func (Interface) Interface

func WithMetrics(m metrics.MetricsConfigManager, t metrics.ClientType) NewOption {
	return func(i Interface) Interface {
		return i.WithMetrics(m, t)
	}
}

func WithTracing() NewOption {
	return func(i Interface) Interface {
		return i.WithTracing()
	}
}

func WithLogging(logger logr.Logger) NewOption {
	return func(i Interface) Interface {
		return i.WithLogging(logger)
	}
}

func NewForConfig(c *rest.Config, opts ...NewOption) (Interface, error) {
	inner, err := {{ Pkg .Target.Type.PkgPath }}.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigAndClient(c *rest.Config, httpClient *http.Client, opts ...NewOption) (Interface, error) {
	inner, err := {{ Pkg .Target.Type.PkgPath }}.NewForConfigAndClient(c, httpClient)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigOrDie(c *rest.Config, opts ...NewOption) Interface {
	return From({{ Pkg .Target.Type.PkgPath }}.NewForConfigOrDie(c), opts...)
}

type wrapper struct {
	{{ GoType .Target.Type }}
}

func from(inner {{ GoType .Target }}, opts ...NewOption) Interface {
	return &wrapper{inner}
}

func (i *wrapper) WithMetrics(m metrics.MetricsConfigManager, t metrics.ClientType) Interface {
	return from(WrapWithMetrics(i, m, t))
}

func (i *wrapper) WithTracing() Interface {
	return from(WrapWithTracing(i))
}

func (i *wrapper) WithLogging(logger logr.Logger) Interface {
	return from(WrapWithLogging(i, logger))
}
`
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

type arg struct {
	reflect.Type
	IsVariadic bool
}

func (a arg) IsError() bool {
	return goType(a.Type) == "error"
}

type ret struct {
	reflect.Type
	IsLast bool
}

func (r ret) IsError() bool {
	return goType(r.Type) == "error"
}

type operation struct {
	reflect.Method
}

func (o operation) HasContext() bool {
	return o.Method.Type.NumIn() > 0 && goType(o.Method.Type.In(0)) == "context.Context"
}

func (o operation) HasError() bool {
	for _, out := range getOuts(o.Method) {
		if goType(out) == "error" {
			return true
		}
	}
	return false
}

type resource struct {
	reflect.Type
	Operations []operation
}

func (r resource) Kind() string {
	return strings.ReplaceAll(r.Type.Name(), "Interface", "")
}

type resourceKey reflect.Method

func (r resourceKey) IsNamespaced() bool {
	return r.Type.NumIn() == 1
}

type client struct {
	reflect.Type
	Resources map[resourceKey]resource
}

type clientset struct {
	reflect.Type
	Clients   map[reflect.Method]client
	Resources map[resourceKey]resource
}

func getIns(in reflect.Method) []reflect.Type {
	var out []reflect.Type
	for i := 0; i < in.Type.NumIn(); i++ {
		out = append(out, in.Type.In(i))
	}
	return out
}

func getOuts(in reflect.Method) []ret {
	var out []ret
	for i := 0; i < in.Type.NumOut(); i++ {
		out = append(out, ret{
			Type:   in.Type.Out(i),
			IsLast: i == in.Type.NumOut()-1,
		})
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

func parseResource(in reflect.Type) resource {
	r := resource{
		Type: in,
	}
	for _, operationMethod := range getMethods(in) {
		o := operation{
			Method: operationMethod,
		}
		r.Operations = append(r.Operations, o)
	}
	return r
}

func parseClient(in reflect.Type) client {
	c := client{
		Type:      in,
		Resources: map[resourceKey]resource{},
	}
	for _, resourceMethod := range getMethods(in) {
		// resource methods return only the resosurce interface type
		if resourceMethod.Type.NumOut() == 1 && resourceMethod.Name != "RESTClient" {
			resourceType := resourceMethod.Type.Out(0)
			r := resource{
				Type: resourceType,
			}
			for _, operationMethod := range getMethods(resourceType) {
				o := operation{
					Method: operationMethod,
				}
				r.Operations = append(r.Operations, o)
			}
			c.Resources[resourceKey(resourceMethod)] = r
		}
	}
	return c
}

func parseClientset(in reflect.Type) clientset {
	cs := clientset{
		Type:      in,
		Clients:   map[reflect.Method]client{},
		Resources: map[resourceKey]resource{},
	}
	for _, clientMethod := range getMethods(in) {
		// client methods return only the client interface type
		if clientMethod.Type.NumOut() == 1 && clientMethod.Name != "Discovery" {
			cs.Clients[clientMethod] = parseClient(clientMethod.Type.Out(0))
		} else if clientMethod.Name == "Discovery" {
			cs.Resources[resourceKey(clientMethod)] = parseResource(clientMethod.Type.Out(0))
		}
	}
	return cs
}

func parseImports(in reflect.Type) []string {
	imports := sets.New(in.PkgPath())
	for _, m := range getMethods(in) {
		for _, i := range getIns(m) {
			if i.Kind() == reflect.Pointer {
				i = i.Elem()
			}
			if i.PkgPath() != "" {
				imports.Insert(i.PkgPath())
			}
		}
		for _, i := range getOuts(m) {
			pkg := i.PkgPath()
			if i.Kind() == reflect.Pointer {
				pkg = i.Elem().PkgPath()
			}
			if pkg != "" {
				imports.Insert(pkg)
			}
		}
	}
	return sets.List(imports)
}

func executeTemplate(tpl string, data interface{}, folder string, file string) {
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
			"Returns": func(in reflect.Method) []ret {
				return getOuts(in)
			},
			"Pkg": func(in string) string {
				return packageAlias(in)
			},
			"GoType": func(in reflect.Type) string {
				return goType(in)
			},
			"Packages": func(in reflect.Type) []string {
				return parseImports(in)
			},
		},
	)
	if tmpl, err := tmpl.Parse(tpl); err != nil {
		panic(err)
	} else {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			panic(fmt.Sprintf("Failed to create directories for %s", folder))
		}
		f, err := os.Create(path.Join(folder, file))
		if err != nil {
			panic(fmt.Sprintf("Failed to create file %s", path.Join(folder, file)))
		}
		if err := tmpl.Execute(f, map[string]interface{}{
			"Folder": folder,
			"Target": data,
		}); err != nil {
			panic(err)
		}
	}
}

func generateResource(r resource, folder string) {
	executeTemplate(resourceTpl, r, folder, "resource.generated.go")
}

func generateClient(c client, folder string) {
	executeTemplate(clientTpl, c, folder, "client.generated.go")
	for m, r := range c.Resources {
		generateResource(r, path.Join(folder, strings.ToLower(m.Name)))
	}
}

func generateClientset(cs clientset, folder string) {
	executeTemplate(clientsetTpl, cs, folder, "clientset.generated.go")
	for m, c := range cs.Clients {
		generateClient(c, path.Join(folder, strings.ToLower(m.Name)))
	}
	for m, r := range cs.Resources {
		generateResource(r, path.Join(folder, strings.ToLower(m.Name)))
	}
}

func generateInterface(cs clientset, folder string) {
	executeTemplate(interfaceTpl, cs, folder, "interface.generated.go")
}

func main() {
	kube := parseClientset(reflect.TypeOf((*kubernetes.Interface)(nil)).Elem())
	generateClientset(kube, "pkg/clients/kube")
	generateInterface(kube, "pkg/clients/kube")
	kyverno := parseClientset(reflect.TypeOf((*versioned.Interface)(nil)).Elem())
	generateClientset(kyverno, "pkg/clients/kyverno")
	generateInterface(kyverno, "pkg/clients/kyverno")
	dynamicInterface := parseClientset(reflect.TypeOf((*dynamic.Interface)(nil)).Elem())
	dynamicResource := parseResource(reflect.TypeOf((*dynamic.ResourceInterface)(nil)).Elem())
	generateResource(dynamicResource, "pkg/clients/dynamic/resource")
	generateInterface(dynamicInterface, "pkg/clients/dynamic")
	metadataInterface := parseClientset(reflect.TypeOf((*metadata.Interface)(nil)).Elem())
	metadataResource := parseResource(reflect.TypeOf((*metadata.ResourceInterface)(nil)).Elem())
	generateInterface(metadataInterface, "pkg/clients/metadata")
	generateResource(metadataResource, "pkg/clients/metadata/resource")
}
