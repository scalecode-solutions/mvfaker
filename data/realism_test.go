package data

import (
	"strings"
	"testing"

	"github.com/tmarq/mvfaker/gen"
)

func TestCityCoherentWithCountry(t *testing.T) {
	mk, err := Build("address.city", nil)
	if err != nil {
		t.Fatal(err)
	}
	usa, _ := placeByCountry("USA")
	g := mk("USA") // dep = country
	for i := 0; i < 100; i++ {
		city := g.Generate(gen.At(uint64(i))).(string)
		if !contains(usa.cities, city) {
			t.Fatalf("city %q not in USA", city)
		}
	}
}

func TestPhoneCoherentWithCountry(t *testing.T) {
	mk, _ := Build("phone", nil)
	g := mk("Germany")
	for i := 0; i < 50; i++ {
		ph := g.Generate(gen.At(uint64(i), 1)).(string)
		if !strings.HasPrefix(ph, "+49 ") {
			t.Fatalf("German phone %q missing +49", ph)
		}
	}
}

func TestZipfSkew(t *testing.T) {
	// the most common entry should dominate a uniform pick
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
