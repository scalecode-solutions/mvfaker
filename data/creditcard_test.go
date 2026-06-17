package data

import (
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/gen"
)

// luhnValid is an INDEPENDENT implementation of the Luhn checksum (not the
// generator's check-digit code), so the test is a real external check.
func luhnValid(s string) bool {
	sum, double := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// Anchor the validator itself against known-good / known-bad numbers, so a bug
// in luhnValid can't make the generator tests pass vacuously.
func TestLuhnValidatorAnchor(t *testing.T) {
	if !luhnValid("4242424242424242") { // a well-known valid Visa test number
		t.Fatal("validator rejects a known-valid number")
	}
	if luhnValid("4242424242424241") {
		t.Fatal("validator accepts a known-invalid number")
	}
}

func TestCreditCardsValid(t *testing.T) {
	cases := []struct {
		scheme  string
		length  int
		prefix1 string
	}{
		{"visa", 16, "4"},
		{"mastercard", 16, "5"},
		{"amex", 15, "3"},
		{"discover", 16, "6"},
	}
	for _, c := range cases {
		mk, err := Build("creditcard."+c.scheme, nil)
		if err != nil {
			t.Fatal(err)
		}
		g := mk(nil)
		for i := 0; i < 300; i++ {
			num := g.Generate(gen.At(uint64(i))).(string)
			if len(num) != c.length {
				t.Fatalf("%s: length %d, want %d (%s)", c.scheme, len(num), c.length, num)
			}
			if !strings.HasPrefix(num, c.prefix1) {
				t.Fatalf("%s: %s missing expected prefix %s", c.scheme, num, c.prefix1)
			}
			if !luhnValid(num) {
				t.Fatalf("%s: %s fails Luhn", c.scheme, num)
			}
		}
	}
}

func TestCreditCardCoherence(t *testing.T) {
	mk, _ := Build("creditcard", nil) // creditcard with a `from` type dependency
	g := mk("American Express")
	for i := 0; i < 100; i++ {
		num := g.Generate(gen.At(uint64(i))).(string)
		if len(num) != 15 || !strings.HasPrefix(num, "3") {
			t.Fatalf("from=Amex produced non-Amex number %q", num)
		}
	}
	gv := mk("Visa")
	for i := 0; i < 100; i++ {
		if num := gv.Generate(gen.At(uint64(i))).(string); !strings.HasPrefix(num, "4") || len(num) != 16 {
			t.Fatalf("from=Visa produced non-Visa number %q", num)
		}
	}
}

func TestCreditCardGeneric(t *testing.T) {
	mk, _ := Build("creditcard", nil)
	g := mk(nil)
	for i := 0; i < 500; i++ {
		num := g.Generate(gen.At(uint64(i))).(string)
		if !luhnValid(num) {
			t.Fatalf("%s fails Luhn", num)
		}
	}
}
