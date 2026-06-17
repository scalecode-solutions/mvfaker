package data

import (
	"fmt"
	"strings"

	"github.com/scalecode-solutions/mvfaker/gen"
)

// Credit-card generators. Numbers are Luhn-valid (they pass the checksum real
// processors use) but use random issuer ranges within each scheme's prefixes —
// they are fake test numbers, never real issued cards.

type cardScheme struct {
	display  string
	prefixes []string
	length   int
}

var cardSchemes = map[string]cardScheme{
	"visa":       {"Visa", []string{"4"}, 16},
	"mastercard": {"Mastercard", []string{"51", "52", "53", "54", "55"}, 16},
	"amex":       {"American Express", []string{"34", "37"}, 15},
	"discover":   {"Discover", []string{"6011", "65"}, 16},
}

var schemeOrder = []string{"visa", "mastercard", "amex", "discover"}

// displayToKey maps "Visa" → "visa" so a number can cohere with a type field.
var displayToKey = map[string]string{}

func init() {
	for k, sc := range cardSchemes {
		displayToKey[sc.display] = k
	}
}

// schemeFor resolves a scheme from a type display name (dep), or picks one.
func schemeFor(dep any, s gen.Source) cardScheme {
	if name, ok := dep.(string); ok {
		if key, found := displayToKey[name]; found {
			return cardSchemes[key]
		}
	}
	return cardSchemes[schemeOrder[s.Draw(uint64(len(schemeOrder)))]]
}

// luhnCheckDigit returns the digit that makes partial+digit pass the Luhn check.
func luhnCheckDigit(partial []int) int {
	sum, double := 0, true // the digit just left of the check digit is doubled
	for i := len(partial) - 1; i >= 0; i-- {
		d := partial[i]
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return (10 - sum%10) % 10
}

func genCard(sc cardScheme, s gen.Source) string {
	prefix := sc.prefixes[s.Draw(uint64(len(sc.prefixes)))]
	digits := make([]int, 0, sc.length)
	for _, r := range prefix {
		digits = append(digits, int(r-'0'))
	}
	for len(digits) < sc.length-1 {
		digits = append(digits, int(s.Draw(10)))
	}
	digits = append(digits, luhnCheckDigit(digits))

	var b strings.Builder
	for _, d := range digits {
		b.WriteByte(byte('0' + d))
	}
	return b.String()
}

func init() {
	for key, sc := range cardSchemes {
		sc := sc
		Register("creditcard."+key, func(Params) (MakeFn, error) {
			g := gen.New(func(s gen.Source) string { return genCard(sc, s) })
			return func(any) gen.Generator[any] { return boxed(g) }, nil
		})
	}

	// creditcard / creditcard.number cohere with a type field via `from`: given a
	// type display name they produce a matching number; otherwise a random scheme.
	cardNumber := func(Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any { return genCard(schemeFor(dep, s), s) })
		}, nil
	}
	Register("creditcard", cardNumber)
	Register("creditcard.number", cardNumber)

	Register("creditcard.type", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			return cardSchemes[schemeOrder[s.Draw(uint64(len(schemeOrder)))]].display
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// CVV length coheres too: American Express uses 4 digits, others 3. An
	// explicit `digits` param overrides.
	Register("creditcard.cvv", func(p Params) (MakeFn, error) {
		forced := p.Int("digits", 0)
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				n := forced
				if n == 0 {
					n = 3
					if name, ok := dep.(string); ok && name == "American Express" {
						n = 4
					}
				}
				var b strings.Builder
				for i := 0; i < n; i++ {
					b.WriteByte(byte('0' + s.Draw(10)))
				}
				return b.String()
			})
		}, nil
	})

	Register("creditcard.expiry", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			return fmt.Sprintf("%02d/%02d", 1+int(s.Draw(12)), 26+int(s.Draw(6))) // MM/YY, near future
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})
}
