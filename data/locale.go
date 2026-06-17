package data

import (
	"embed"
	"encoding/json"
	"sort"
)

// Locale is a drop-in unit of region-specific data. To add a locale, drop a JSON
// file in data/locales/ — no Go changes needed (go:embed picks it up at build).
// Fields may be partial: anything omitted falls back to the default locale
// (en-US). `country` is the ISO alpha-2 this locale is for, so address fields can
// cohere with a generated country. See CONTRIBUTING.md.
type Locale struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Country      string   `json:"country"` // ISO alpha-2
	FirstNames   []string `json:"firstNames"`
	LastNames    []string `json:"lastNames"`
	Cities       []string `json:"cities"`
	Regions      []string `json:"regions"`
	Streets      []string `json:"streets"`
	PostalFormat string   `json:"postalFormat"`
}

//go:embed locales/*.json
var localeFS embed.FS

const defaultLocaleCode = "en-US"

var (
	localesByCode = map[string]*Locale{}
	localesByA2   = map[string]*Locale{}
)

func init() {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		panic("mvfaker: cannot read embedded locales: " + err.Error())
	}
	for _, e := range entries {
		b, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			panic("mvfaker: cannot read locale " + e.Name() + ": " + err.Error())
		}
		var l Locale
		if err := json.Unmarshal(b, &l); err != nil {
			panic("mvfaker: bad locale JSON in " + e.Name() + ": " + err.Error())
		}
		if l.Code == "" {
			panic("mvfaker: locale " + e.Name() + " has no \"code\"")
		}
		localesByCode[l.Code] = &l
		if l.Country != "" {
			localesByA2[l.Country] = &l
		}
	}
	if localesByCode[defaultLocaleCode] == nil {
		panic("mvfaker: missing default locale " + defaultLocaleCode)
	}
}

func defaultLocale() *Locale { return localesByCode[defaultLocaleCode] }

// localeFor returns the locale for a code, or the default if unknown/empty.
func localeFor(code string) *Locale {
	if l, ok := localesByCode[code]; ok {
		return l
	}
	return defaultLocale()
}

// localeForCountry returns the locale bound to an ISO alpha-2, or nil.
func localeForCountry(a2 string) *Locale { return localesByA2[a2] }

// firstNamesOf / lastNamesOf fall back to the default locale's names when a
// locale doesn't provide its own yet (the common case for a new contribution).
func (l *Locale) firstNamesOf() []string {
	if len(l.FirstNames) > 0 {
		return l.FirstNames
	}
	return defaultLocale().FirstNames
}

func (l *Locale) lastNamesOf() []string {
	if len(l.LastNames) > 0 {
		return l.LastNames
	}
	return defaultLocale().LastNames
}

// LocaleCodes lists the loaded locale codes (sorted) — handy for a manifest.
func LocaleCodes() []string {
	out := make([]string, 0, len(localesByCode))
	for k := range localesByCode {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
