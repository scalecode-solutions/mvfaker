# mvfaker

[![CI](https://github.com/scalecode-solutions/mvfaker/actions/workflows/ci.yml/badge.svg)](https://github.com/scalecode-solutions/mvfaker/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/scalecode-solutions/mvfaker.svg)](https://pkg.go.dev/github.com/scalecode-solutions/mvfaker)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

One fake-data engine, four front doors — driven by a shared set of recipes.

```
mvfaker --fixt example.hcl     # a few repeatable example records
mvfaker --mock example.hcl     # realistic records, fresh-looking
mvfaker --mock --serve :8080 example.hcl   # …or a stand-in HTTP API
mvfaker --seed --sql example.hcl       # dataset → SQL INSERTs (SQLite/MySQL/Postgres)
mvfaker --seed --copy example.hcl      # dataset → Postgres COPY (fast bulk load)
mvfaker --seed --ndjson out example.hcl # dataset → out/<entity>.ndjson (MongoDB etc.)
mvfaker --check --schema schema.sql example.hcl  # verify config vs schema; emits nothing
mvfaker --prop                 # run registered property rules, shrink failures
mvfaker --prop demo.no-big     # …or one named rule
mvfaker --gen -pkg fixtures example.hcl > fixtures.go  # compile to Go (~11× faster seeding)
```

## Usage

### Install

```bash
go install github.com/scalecode-solutions/mvfaker/cmd/mvfaker@latest
# …or from a clone:
go build -o mvfaker ./cmd/mvfaker
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
for the full showcase — the same config seeded into **Postgres, SQLite, MySQL/CSV,
and MongoDB**, with live Go API backends.)

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

### Importing it in another project

```bash
go get github.com/scalecode-solutions/mvfaker@latest
```

### Use it from an AI agent (MCP server)

mvfaker ships a [Model Context Protocol](https://modelcontextprotocol.io) server
(pure Go, no Node/npm) so an agent can discover and drive it at inference time —
no waiting to be in the model's training data.

```bash
go install github.com/scalecode-solutions/mvfaker/cmd/mvfaker-mcp@latest
```

Register it (Claude Code):

```bash
claude mcp add mvfaker mvfaker-mcp
```

…or in an MCP `config` / `.mcp.json`:

```json
{ "mcpServers": { "mvfaker": { "command": "mvfaker-mcp" } } }
```

Tools exposed:

| Tool | What it does |
|---|---|
| `list_generators` | the generator catalog (so the agent knows what's available) |
| `list_locales` | available locale codes |
| `generate_dataset` | turn a JSON dataset spec (entities → fields with `gen`/`from`/`ref`/`unique`) into data (`json`/`sql`/`copy`) — coherent and referentially valid by construction |

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

Names — `name.first/last/full` — are drawn from **US Census surnames (top 1,000)
and SSA first names (top 600), weighted by real frequency**, so common names
dominate like they do in reality (~600k unique full names).

Geography is backed by a **249-country dataset** (ISO codes, calling codes,
currencies, capitals, continents): `country`, `country.code`, `country.code3`,
`country.callingcode`, `country.currency`, `country.capital`, `country.continent`,
`currency.code`, plus `address.city/region/postal/full`, `phone`, and US
`us.state` / `us.state.code`.

Network: `ipv4` (public), `ipv4.private` (RFC 1918), `ipv6` (canonical RFC 5952,
with `::` compression), `mac`.

Payments — `creditcard` (Luhn-valid), `creditcard.number`, `creditcard.type`,
`creditcard.visa/mastercard/amex/discover`, `creditcard.cvv`, `creditcard.expiry`.
Coherent: `number` and `cvv` with `from = "type"` match the scheme (Amex → 15
digits + 4-digit CVV; Visa/MC/Discover → 16 + 3). Fake test numbers, never real.

Also: `internet.email`, `date`, `datetime`, `timestamp` (now-anchored, day
granularity), `money`/`price`, `number`, `bool`, `uuid`, `lorem.word(s)`,
`json` (jsonb columns), `password.bcrypt` (login-capable seed users),
`oneof` (`values=[…]` + optional `weights=[…]` — define your own categorical
data), and `copy` (echo a `from` field).

**Field modifiers** apply to any field after generation: `transform` (`lower`/
`upper`/`slug`/`title`), `maxlen` (truncate to a `varchar(n)`), `null_prob`
(emit `NULL` with a probability), and `when` (NULL unless a condition on a
sibling holds — `when = "state == deactivated"`, supports `==`/`!=`/`in [..]`).
Combine `copy` + `transform` for derived columns, e.g. `handle_lower =
lower(handle)`.

**Cross-entity coherence** — a field can equal a *referenced* row's value:
`from = "user_id.email"` re-derives the row that the `user_id` FK points at and
projects its `email` (so `auth.uname == users.email`, exact, no join). Coherent
by construction — positional determinism means the re-derived row is the stored
one, uniqueness suffix and all.

Everything coheres via `from`: set a `country` field, then `from = "country"` on
`country.code` / `currency` / `city` / `phone` and they all match. (Reserved
attribute names: `gen`, `from`, `ref`, `unique` — generator params use other
names, e.g. `date` takes `min`/`max` years.) Data sources: [ATTRIBUTION.md](ATTRIBUTION.md).

### Locales

Region-specific data (names, cities, regions, postal formats) lives in drop-in
JSON files under [`data/locales/`](data/locales/) — `go:embed`'d at build, no Go
required to add one. Pass `locale = "ja-JP"` to `name.*`/`address.*`, or let
`from = "country"` pick the locale automatically. Partial locales fall back to
`en-US`, so "just the cities and postal format for my country" is a valid PR.
**Adding your locale: [CONTRIBUTING.md](CONTRIBUTING.md).**

## Status & license

`v0` — API and seeded output not yet frozen. Rough edges welcome.
Licensed under the [MIT License](LICENSE).

## Layout

```
gen/          entropy (Source) + pure Generator[T] + combinators + tree shrinker
data/         built-in generators, locales, and the name→registry
schema/       entities, FK runner, uniqueness, sinks (JSON/SQL/COPY), HCL front-end
mock/         --mock --serve HTTP stand-in API
codegen/      --gen: compile a config to standalone Go (scale path)
fill.go       struct-tag front-end (mvfaker.Fill / Struct[T])
rule.go       property-rule registry
cmd/mvfaker/      the CLI (--fixt/--mock/--seed/--prop/--gen/--check)
cmd/mvfaker-mcp/  MCP server (Go) — drive mvfaker from an AI agent
integration/  showcase: same config → Postgres, SQLite, MySQL/CSV, MongoDB
examples/     code-built plan + embedded-document generation
```
