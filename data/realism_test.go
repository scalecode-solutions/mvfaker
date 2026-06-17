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
	us := details["US"].cities
	for i := 0; i < 100; i++ {
		city := g.Generate(gen.At(uint64(i))).(string)
		if !contains(us, city) {
			t.Fatalf("US city %q not in detailed set %v", city, us)
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
	if len(firstNames) < 500 || len(lastNames) < 900 || len(countries) < 240 {
		t.Fatalf("dataset too small: first=%d last=%d countries=%d",
			len(firstNames), len(lastNames), len(countries))
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
