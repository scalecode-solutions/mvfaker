# Data attribution

mvfaker's built-in datasets (`data/dataset_gen.go`) are derived from public
sources. mvfaker's own code is MIT (see [LICENSE](LICENSE)); the bundled data
carries the licenses below.

- **Surnames** — US Census Bureau most-common-surnames, via
  [FiveThirtyEight/data](https://github.com/fivethirtyeight/data/tree/master/most-common-name)
  (`surnames.csv`). Licensed **CC BY 4.0**. Top 1,000 surnames, ranked by census
  frequency.
- **First names** — US Social Security Administration baby-name frequencies, via
  [hadley/data-baby-names](https://github.com/hadley/data-baby-names). SSA data is
  US Government work (public domain). Top 600 first names by aggregate frequency.
- **Country codes** — ISO 3166 country names/codes, ITU calling codes, ISO 4217
  currencies, capitals and continents, via
  [datasets/country-codes](https://github.com/datasets/country-codes)
  (Public Domain Dedication and License). 249 countries.
- **US states** — names and USPS abbreviations (public-domain facts).

Country/ISO codes, calling codes and currency codes are factual data and not
themselves subject to copyright.

## Locale files (`data/locales/`)

- **en-US** is built from the census/SSA sources above (real frequency ranking).
- **Other locales** (`es-ES`, `es-MX`, `fr-FR`, `it-IT`, `de-DE`, `en-GB`,
  `ja-JP`, `pt-BR`) are **bootstrapped from common-knowledge public facts** —
  widely-documented top surnames, common given names, major cities, regions and
  postal formats. The data is real, but frequency *ordering* is approximate
  (not yet sourced from each country's census). **Native speakers: corrections
  and real frequency data are very welcome** — see [CONTRIBUTING.md](CONTRIBUTING.md).
