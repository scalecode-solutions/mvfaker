package data

import (
	"fmt"
	"time"

	"github.com/scalecode-solutions/mvfaker/gen"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	// copy: returns the value of the field named in `from`, unchanged. Combine
	// with a transform modifier for derived columns, e.g. a case-folded index key:
	//   gen = "copy", from = "handle", transform = "lower"   ->  lower(handle)
	Register("copy", func(Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(gen.Source) any { return dep })
		}, nil
	})

	// oneof: pick from an explicit set, optionally weighted. Lets configs define
	// their own categorical data inline (state, platform, role…).
	//   gen = "oneof", values = ["active","inactive","deleted"], weights = [8,1,1]
	Register("oneof", func(p Params) (MakeFn, error) {
		vals, ok := p["values"].([]any)
		if !ok || len(vals) == 0 {
			return nil, fmt.Errorf("oneof requires a non-empty values=[...]")
		}
		weights, _ := p["weights"].([]any)
		w := make([]int, len(vals))
		total := 0
		for i := range w {
			wi := 1
			if i < len(weights) {
				switch x := weights[i].(type) {
				case int:
					wi = x
				case int64:
					wi = int(x)
				case float64:
					wi = int(x)
				}
			}
			if wi < 0 {
				wi = 0
			}
			w[i] = wi
			total += wi
		}
		g := gen.New(func(s gen.Source) any {
			if total <= 0 {
				return vals[s.Draw(uint64(len(vals)))]
			}
			r := int(s.Draw(uint64(total)))
			for i := range w {
				if r < w[i] {
					return vals[i]
				}
				r -= w[i]
			}
			return vals[len(vals)-1]
		})
		return func(any) gen.Generator[any] { return g }, nil
	})

	// timestamp: a now-anchored timestamp some days/seconds in the past, with
	// day granularity. Deterministic: anchored to a fixed `now` (override with
	// now="2026-06-17T00:00:00Z"). Output is Postgres timestamp text.
	//   gen = "timestamp", days_ago_min = 0, days_ago_max = 200
	Register("timestamp", func(p Params) (MakeFn, error) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		if ns := p.Str("now", ""); ns != "" {
			if t, err := time.Parse(time.RFC3339, ns); err == nil {
				now = t
			}
		}
		lo, hi := p.Int("days_ago_min", 0), p.Int("days_ago_max", 365)
		if hi < lo {
			lo, hi = hi, lo
		}
		span := hi - lo + 1
		g := gen.New(func(s gen.Source) string {
			days := lo + int(s.Draw(uint64(span)))
			secs := int(s.Draw(86400))
			t := now.Add(-time.Duration(days)*24*time.Hour - time.Duration(secs)*time.Second)
			return t.Format("2006-01-02 15:04:05")
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// json: a small valid JSON object — for jsonb columns.
	Register("json", func(p Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			k := words[s.Draw(uint64(len(words)))]
			v := words[s.Draw(uint64(len(words)))]
			return fmt.Sprintf(`{%q: %q, "n": %d, "active": %t}`, k, v, s.Draw(100), s.Draw(2) == 0)
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// password.bcrypt: a bcrypt hash of a known plaintext, computed once and
	// reused for all rows (bcrypt is slow + non-deterministic). Seeded users can
	// log in with the plaintext.  gen = "password.bcrypt", plaintext = "hunter2"
	Register("password.bcrypt", func(p Params) (MakeFn, error) {
		plain := p.Str("plaintext", "password")
		cost := p.Int("cost", bcrypt.DefaultCost)
		hash, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
		if err != nil {
			return nil, err
		}
		h := string(hash)
		return func(any) gen.Generator[any] { return boxed(gen.Const(h)) }, nil
	})
}
