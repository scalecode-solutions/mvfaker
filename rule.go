package mvfaker

import (
	"fmt"
	"sort"
	"sync"

	"github.com/scalecode-solutions/mvfaker/gen"
)

// RuleResult reports a property check in a type-erased form for the CLI.
type RuleResult struct {
	Name    string
	Failed  bool
	Example string // string form of the shrunk counterexample
	Runs    int
	Shrinks int
}

type ruleFn func(seed uint64, runs int) RuleResult

var (
	ruleMu sync.RWMutex
	rules  = map[string]ruleFn{}
)

// RegisterRule binds a named property: a generator of inputs plus a predicate
// that must hold for all of them. This is the seam --prop runs through — a rule
// is logic, so it lives in code and is referenced by name, exactly like a
// custom generator. The generic type is erased into a closure so rules of
// different shapes share one registry.
func RegisterRule[T any](name string, g gen.Generator[T], prop func(T) bool) {
	ruleMu.Lock()
	defer ruleMu.Unlock()
	rules[name] = func(seed uint64, runs int) RuleResult {
		r := gen.Check(seed, runs, g, prop)
		return RuleResult{
			Name: name, Failed: r.Failed,
			Example: fmt.Sprintf("%v", r.Value), Runs: r.Runs, Shrinks: r.Shrinks,
		}
	}
}

// RunRule runs one registered rule.
func RunRule(name string, seed uint64, runs int) (RuleResult, error) {
	ruleMu.RLock()
	fn, ok := rules[name]
	ruleMu.RUnlock()
	if !ok {
		return RuleResult{}, fmt.Errorf("unknown rule %q (have: %v)", name, RuleNames())
	}
	return fn(seed, runs), nil
}

// RunAllRules runs every registered rule, sorted by name.
func RunAllRules(seed uint64, runs int) []RuleResult {
	names := RuleNames()
	out := make([]RuleResult, 0, len(names))
	for _, n := range names {
		r, _ := RunRule(n, seed, runs)
		out = append(out, r)
	}
	return out
}

// RuleNames lists registered rules, sorted.
func RuleNames() []string {
	ruleMu.RLock()
	defer ruleMu.RUnlock()
	out := make([]string, 0, len(rules))
	for k := range rules {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
