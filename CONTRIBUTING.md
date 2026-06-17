# Contributing to mvfaker

## Adding a locale (no Go required)

mvfaker's region-specific data lives in **drop-in JSON files** under
[`data/locales/`](data/locales/). To add your country/language, add one file —
the build picks it up automatically (`go:embed`), no code changes.

### 1. Create `data/locales/<code>.json`

Use a BCP-47-ish code (`ja-JP`, `fr-FR`, `es-MX`). Start from
[`data/locales/en-US.json`](data/locales/en-US.json) or this template:

```json
{
  "code": "fr-FR",
  "name": "French (France)",
  "country": "FR",
  "firstNames": ["Jean", "Marie", "Pierre"],
  "lastNames": ["Martin", "Bernard", "Dubois"],
  "cities": ["Paris", "Lyon", "Marseille"],
  "regions": ["Île-de-France", "Provence"],
  "streets": ["Rue de la Paix", "Avenue des Champs"],
  "postalFormat": "#####"
}
```

| Field | Meaning |
|---|---|
| `code` | locale id (required, unique) |
| `name` | human label |
| `country` | ISO alpha-2 this locale is for — links it to the country dataset so addresses cohere (`from = "country"`) |
| `firstNames` / `lastNames` | name pools, **ordered most-common first** (the generator weights by rank) |
| `cities` / `regions` / `streets` | address parts |
| `postalFormat` | pattern: `#` = digit, `@` = uppercase letter (e.g. `@@## #@@`) |

**Partial locales are fine.** Omit anything you don't have — `firstNames`,
`cities`, etc. fall back to `en-US`. A locale that only adds correct cities and a
postal format is a perfectly good first PR.

### 2. Verify

```bash
go test ./data/        # locale loads + integrity checks run here
go build -o mvfaker ./cmd/mvfaker
./mvfaker --fixt -n 5 <(echo 'entity "p" {
  field "name" { gen = "name.full"
    locale = "fr-FR" }
}
dataset "d" { counts = { p = 5 } }')
```

### Notes

- **Order names by frequency** (most common first) — the generator is Zipf-weighted,
  so ranking matters for realism.
- **Data must be freely licensed.** Public-domain or permissive sources only; add
  a line to [ATTRIBUTION.md](ATTRIBUTION.md) for non-trivial datasets.
- Keep it ASCII-safe where possible, but UTF-8 is fine (it's embedded as-is).

## Code changes

`go build ./... && go vet ./... && go test ./...` must pass (CI enforces it).
The architecture is documented in [DESIGN.md](DESIGN.md) — new generators are
registered in `data/` via `Register`, and should support `from` coherence where
it makes sense.
