package data

import (
	"fmt"
	"sort"

	"github.com/tmarq/mvfaker/gen"
)

// Params are the declarative attributes from a config field (or a code call).
type Params map[string]any

// MakeFn builds a value generator, optionally using the value of a field this
// one derives from (dep is nil when there is no `from`).
type MakeFn func(dep any) gen.Generator[any]

// Builder turns params into a MakeFn. This is the registry's unit: config names
// a builder; code can register new ones. The seam between the two front doors.
type Builder func(p Params) (MakeFn, error)

var registry = map[string]Builder{}

// Register adds a named builder. Anything config can't express (custom logic)
// is written in code, registered here, and referenced by name.
func Register(name string, b Builder) { registry[name] = b }

// Build resolves a named builder with params.
func Build(name string, p Params) (MakeFn, error) {
	b, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown generator %q (have: %v)", name, Names())
	}
	return b(p)
}

// Names lists registered builders, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- Param readers ---------------------------------------------------------

func (p Params) Int(key string, def int) int {
	switch v := p[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}

func (p Params) Float(key string, def float64) float64 {
	switch v := p[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return def
}

func (p Params) Str(key, def string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return def
}

func boxed[T any](g gen.Generator[T]) gen.Generator[any] {
	return gen.Map(g, func(v T) any { return v })
}
