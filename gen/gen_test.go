package gen

import "testing"

func TestPositionalDeterministic(t *testing.T) {
	a := At(42, 7, 3).Draw(1000)
	b := At(42, 7, 3).Draw(1000)
	if a != b {
		t.Fatalf("positional source not deterministic: %d != %d", a, b)
	}
	c := At(42, 7, 4).Draw(1000)
	if a == c {
		t.Fatalf("different paths produced same draw (%d)", a)
	}
}

func TestParallelIndependence(t *testing.T) {
	// row 1,000,000 must be reachable without generating the prior rows
	far := At(1, 1_000_000).Draw(100)
	again := At(1, 1_000_000).Draw(100)
	if far != again {
		t.Fatalf("positional addressing not stable: %d != %d", far, again)
	}
}

func TestCheckShrinksToMinimal(t *testing.T) {
	g := Slice(IntRange(0, 8), IntRange(0, 1000))
	prop := func(xs []int) bool {
		for _, x := range xs {
			if x >= 900 {
				return false
			}
		}
		return true
	}
	res := Check(1, 1000, g, prop)
	if !res.Failed {
		t.Fatal("expected property to fail")
	}
	if len(res.Value) != 1 {
		t.Fatalf("expected single-element counterexample, got %v", res.Value)
	}
	if res.Value[0] < 900 {
		t.Fatalf("counterexample element should be >= 900, got %v", res.Value)
	}
}
