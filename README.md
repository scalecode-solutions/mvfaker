# mvfaker

One fake-data engine, four front doors — driven by a shared set of recipes.

```
mvfaker --fixt example.hcl     # a few repeatable example records
mvfaker --mock example.hcl     # realistic records, fresh-looking
mvfaker --seed --sql example.hcl   # a full, internally-consistent dataset → SQL
mvfaker --prop                 # run registered property rules, shrink failures
mvfaker --prop demo.no-big     # …or one named rule
```

Register property rules in code (the registry seam):

```go
mvfaker.RegisterRule("no-big",
    gen.Slice(gen.IntRange(0, 8), gen.IntRange(0, 1000)),
    func(xs []int) bool {
        for _, x := range xs { if x >= 900 { return false } }
        return true
    })
// --prop shrinks a failure to the simplest case, e.g. [900]
```

## Why it's different

- **Coherent records.** `email` derives from `name` (`Michael Nguyen` →
  `michael.nguyen@…`), not random per-field garbage. Coherence is just `Bind`.
- **Uniqueness without a mutable set.** `unique = true` is enforced by the runner
  from the row index (Feistel permutation for ints, index-derived tag for
  strings), so it stays parallel and deterministic — 50k rows, 50k distinct
  emails, identical across runs.
- **Pure value layer, stateful dataset layer.** Generators are pure functions
  `entropy → value`; all cross-row state (FKs, uniqueness, ordering) lives above
  them. That purity is what keeps shrinking, replay, and parallel seeding alive.
- **Splittable entropy.** One primitive (`Draw` + `Split`) under everything.
  Positional addressing makes seeding parallel and reproducible; a recording
  variant gives generic shrinking for property tests — same generators, swapped
  dice.
- **Config and code are two faces of one engine.** HCL parses into the exact
  objects code builds. A registry is the seam: anything config can't express is
  registered in code and referenced by name.

## Code-side: fill a struct

```go
type User struct {
    Name  string `fake:"name.full"`
    Email string `fake:"internet.email,from=Name"` // coherence
    Age   int    `fake:"number,min=18,max=90"`
}
var u User
mvfaker.FillAt(&u, 1)         // deterministic
g := mvfaker.Struct[User]()   // …or a composable Generator[User]
```

Untagged fields are inferred (by name, then Go kind); nested structs and slices
fill automatically. `Struct[T]()` is an ordinary generator, so it composes with
`gen.Slice`, `gen.Optional`, and the rest.

See [DESIGN.md](DESIGN.md) for the full architecture.

## Status

`v0` — internal, API and seeded output not yet frozen. Rough edges welcome.

## Layout

```
gen/      entropy (Source) + pure Generator[T] + combinators + shrinker
data/     built-in generators and the name→registry
schema/   entities, FK runner, sinks (JSON/SQL), HCL front-end
cmd/      the CLI
```
