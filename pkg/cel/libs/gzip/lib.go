// Package gzip is a bounded local copy of github.com/kyverno/sdk's gzip CEL library:
// identical function/overload names and version so it is a drop-in replacement, but
// gzip.decompress caps its output instead of reading the whole decompressed stream. cel-go
// bills a step only after it returns, so pricing alone leaves the transient allocation of
// the whole decompressed payload reachable per request (a 99.6 KB input can decompress to
// 357 MB at cost 5); the cap bounds that allocation directly. cel-go rejects re-binding an
// existing overload and the SDK module has no replace directive, which is why this is a
// local copy rather than a wrapper.
package gzip

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/sdk/extensions/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.gzip"

type lib struct {
	version        *version.Version
	maxOutputBytes uint64
}

// Latest matches the SDK library's own Latest(), so the version-gate story is identical.
func Latest() *version.Version {
	return versions.KyvernoLatest
}

// Lib constructs the local, bounded gzip library. Same function names and overload IDs as
// github.com/kyverno/sdk/extensions/cel/libs/gzip: decompress_bytes_string,
// compress_any_bytes. maxOutputBytes bounds gzip.decompress's output; 0 falls back to the
// default (10 MB, matching the per-program cost limit's own maxStepBytes at defaults).
// Carried on the lib instance and closed over by impl, not a package-level variable: envs
// are built concurrently, and a shared mutable global written on every env construction is
// a data race regardless of whether every writer stores the same value.
func Lib(v *version.Version, maxOutputBytes uint64) cel.EnvOption {
	if v == nil {
		panic(libraryName + ": library version must not be nil")
	}
	if maxOutputBytes == 0 {
		maxOutputBytes = 10_000_000
	}
	return cel.Lib(&lib{version: v, maxOutputBytes: maxOutputBytes})
}

func (*lib) LibraryName() string { return libraryName }

func (l *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{l.extendEnv}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// extendEnv is itself a cel.EnvOption (returned from CompileOptions above), applied as part
// of the single initial env build, not a second Extend called after the env has already
// been handed back to a caller.
func (l *lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	adapter := env.CELTypeAdapter()
	impl := impl{Adapter: adapter, maxOutputBytes: l.maxOutputBytes}
	return env.Extend(
		cel.Function("gzip.decompress", cel.Overload("decompress_bytes_string", []*cel.Type{types.BytesType}, types.StringType, cel.UnaryBinding(impl.decompress_string))),
		cel.Function("gzip.compress", cel.Overload("compress_any_bytes", []*cel.Type{types.DynType}, types.BytesType, cel.UnaryBinding(impl.compress))),
	)
}
