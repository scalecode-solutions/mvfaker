package data

import (
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/gen"
)

func TestCityCoherentWithCountry(t *testing.T) {
	mk, err := Build("address.city", nil)
	if err != nil {
		t.Fatal(err)
	}
	g := mk("United States") // dep = a country name from the dataset
	us := localeForCountry("US").Cities
	for i := 0; i < 100; i++ {
		city := g.Generate(gen.At(uint64(i))).(string)
		if !contains(us, city) {
			t.Fatalf("US city %q not in en-US locale %v", city, us)
		}
	}
}

func TestCallingCodeCoherentWithCountry(t *testing.T) {
	mk, _ := Build("phone", nil)
	g := mk("Germany")
	for i := 0; i < 50; i++ {
		ph := g.Generate(gen.At(uint64(i), 1)).(string)
		if !strings.HasPrefix(ph, "+49 ") {
			t.Fatalf("German phone %q missing +49", ph)
		}
	}
}

func TestCountryCodeCoherence(t *testing.T) {
	mk, _ := Build("country.code", nil)
	if got := mk("Japan").Generate(gen.At(1)).(string); got != "JP" {
		t.Fatalf("Japan code = %q, want JP", got)
	}
	cur, _ := Build("country.currency", nil)
	if got := cur("Japan").Generate(gen.At(1)).(string); got != "JPY" {
		t.Fatalf("Japan currency = %q, want JPY", got)
	}
}

func TestDatasetSize(t *testing.T) {
	d := defaultLocale()
	if len(d.FirstNames) < 500 || len(d.LastNames) < 900 || len(countries) < 240 {
		t.Fatalf("dataset too small: first=%d last=%d countries=%d",
			len(d.FirstNames), len(d.LastNames), len(countries))
	}
}

func TestLocalesLoad(t *testing.T) {
	codes := LocaleCodes()
	if len(codes) < 5 {
		t.Fatalf("expected >=5 locales, got %v", codes)
	}
	// every locale bound to a country must reference a real country
	for _, code := range codes {
		l := localeFor(code)
		if l.Country != "" {
			if _, ok := countryByA2[l.Country]; !ok {
				t.Fatalf("locale %s references unknown country %q", code, l.Country)
			}
		}
	}
}

func TestLocaleNameFallback(t *testing.T) {
	// de-DE has no names yet → name.full should fall back to en-US names, not crash
	mk, err := Build("name.full", Params{"locale": "de-DE"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mk(nil).Generate(gen.At(1)).(string); got == "" {
		t.Fatal("locale name fallback produced empty name")
	}
}

func TestLocaleCityCoherence(t *testing.T) {
	// a JP-bound country should yield JP cities via the ja-JP locale
	mk, _ := Build("address.city", nil)
	jp := localeForCountry("JP").Cities
	for i := 0; i < 50; i++ {
		if city := mk("Japan").Generate(gen.At(uint64(i))).(string); !contains(jp, city) {
			t.Fatalf("Japan city %q not in ja-JP locale %v", city, jp)
		}
	}
}

func TestZipfSkew(t *testing.T) {
	g := zipfPick([]string{"common", "rare1", "rare2", "rare3", "rare4", "rare5"})
	counts := map[string]int{}
	for i := 0; i < 2000; i++ {
		counts[g.Generate(gen.At(uint64(i), 7))]++
	}
	if counts["common"] < 600 {
		t.Fatalf("expected 'common' to dominate, got %d/2000", counts["common"])
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
