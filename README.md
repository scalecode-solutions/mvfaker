# mvfaker

One fake-data engine, four front doors — driven by a shared set of recipes.

```
mvfaker --fixt example.hcl     # a few repeatable example records
mvfaker --mock example.hcl     # realistic records, fresh-looking
mvfaker --mock --serve :8080 example.hcl   # …or a stand-in HTTP API
mvfaker --seed --sql example.hcl    # dataset → SQL INSERTs
mvfaker --seed --copy example.hcl   # dataset → Postgres COPY (fast bulk load)
mvfaker --prop                 # run registered property rules, shrink failures
mvfaker --prop demo.no-big     # …or one named rule
mvfaker --gen -pkg fixtures example.hcl > fixtures.go  # compile to Go (~11× faster seeding)
```

## Usage

### Install

```bash
go build -o mvfaker ./cmd/mvfaker      # or: go install ./cmd/mvfaker
```

### 1. Write a config (`example.hcl`)

Entities, fields, and how they relate. Fields cohere via `from`; foreign keys via
`ref`; dataset sizes via `counts`.

```hcl
entity "users" {
  field "name"  { gen = "name.full" }
  field "email" {
    gen    = "internet.email"
    from   = "name"          # email derives from the name
    unique = true            # guaranteed unique, no mutable set
  }
  field "country" { gen = "address.country" }
  field "city"    { gen = "address.city", from = "country" }  # city matches country
}

entity "posts" {
  field "author_id" { ref = "users.id" }   # FK into users
  field "title"     { gen = "lorem.words", n = 5 }
}

dataset "demo" {
  counts = { users = 1000, posts = 5000 }
}
```

### 2. Run a mode

```bash
mvfaker --fixt -n 5 example.hcl                # a few records → JSON (eyeball / fixtures)
mvfaker --seed --copy example.hcl > seed.sql   # full dataset → Postgres COPY
mvfaker --mock --serve :8080 example.hcl       # fake API: curl localhost:8080/users/3
mvfaker --gen -pkg fixtures example.hcl > fixtures.go   # compile to Go (~11× faster seeding)
```

Load a seed into Postgres: `psql mydb -f seed.sql`. (See [`integration/`](integration/)
for a full Dockerized Go-backend-+-Postgres example seeded entirely by mvfaker.)

### 3. …or use it as a Go library

Fill a struct from tags (untagged fields are inferred):

```go
type User struct {
    Name  string `fake:"name.full"`
    Email string `fake:"internet.email,from=Name"` // coherence
    Age   int    `fake:"number,min=18,max=65"`
}
var u User
mvfaker.FillAt(&u, 42)   // deterministic; mvfaker.Fill(&u) for random
```

Property-test in plain code (drop into a `_test.go`):

```go
res := gen.Check(1, 1000, gen.List(8, gen.IntRange(0, 1000)), func(xs []int) bool {
    for _, x := range xs { if x >= 900 { return false } }
    return true
})
// res.Value → [900]  (the shrunk counterexample)
```

Or register named rules so `--prop` can run them:

```go
mvfaker.RegisterRule("no-big",
    gen.List(8, gen.IntRange(0, 1000)),
    func(xs []int) bool {
        for _, x := range xs { if x >= 900 { return false } }
        return true
    })
```

### Importing it from another (private) project

The repo is private, so tell Go not to use the public proxy:

```bash
export GOPRIVATE=github.com/scalecode-solutions/*
go get github.com/scalecode-solutions/mvfaker
```

### Which mode for what

| You want… | Use |
|---|---|
| A few records to look at / test fixtures | `--fixt` or `mvfaker.Fill(&struct)` |
| A fake API for frontend dev | `--mock --serve :8080` |
| Fill a database | `--seed --copy` → `psql -f` |
| Find edge-case bugs in your code | `gen.Check(...)` in a test, or `--prop` |
| Seed *millions* of rows fast | `--gen` once, call the generated `SeedAll()` |

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

## Generators

`name.first/last/full` (Zipf-weighted), `internet.email`, `address.country/city/
region/postal/full`, `phone`, `date`, `datetime`, `money`/`price`, `number`,
`bool`, `uuid`, `lorem.word(s)`. Locale fields cohere via `from`: set a `country`
field, then `from = "country"` on `city`/`postal`/`phone` and they match it.
(Reserved attribute names: `gen`, `from`, `ref`, `unique` — generator params use
other names, e.g. `date` takes `min`/`max` years.)

## Status

`v0` — internal, API and seeded output not yet frozen. Rough edges welcome.

## Layout

```
gen/          entropy (Source) + pure Generator[T] + combinators + tree shrinker
data/         built-in generators, locales, and the name→registry
schema/       entities, FK runner, uniqueness, sinks (JSON/SQL/COPY), HCL front-end
mock/         --mock --serve HTTP stand-in API
codegen/      --gen: compile a config to standalone Go (scale path)
fill.go       struct-tag front-end (mvfaker.Fill / Struct[T])
rule.go       property-rule registry
cmd/mvfaker/  the CLI (--fixt/--mock/--seed/--prop/--gen)
integration/  Dockerized Go backend + Postgres, seeded by mvfaker
```
