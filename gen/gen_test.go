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

// Two independent lists; only xs matters to the property. The tree shrinker must
// prune the irrelevant ys to empty AND reduce xs to a single element — which
// needs deleting elements from the middle, exactly what a flat tape can't do.
func TestTreeShrinkStructural(t *testing.T) {
	type pair struct{ xs, ys []int }
	g := Bind(List(6, IntRange(0, 1000)), func(xs []int) Generator[pair] {
		return Map(List(6, IntRange(0, 1000)), func(ys []int) pair { return pair{xs, ys} })
	})
	prop := func(p pair) bool {
		for _, x := range p.xs {
			if x >= 900 {
				return false
			}
		}
		return true
	}
	res := Check(1, 2000, g, prop)
	if !res.Failed {
		t.Fatal("expected failure")
	}
	if len(res.Value.ys) != 0 {
		t.Fatalf("irrelevant list should shrink to empty, got %v", res.Value.ys)
	}
	if len(res.Value.xs) != 1 || res.Value.xs[0] < 900 {
		t.Fatalf("xs should shrink to a single >=900 element, got %v", res.Value.xs)
	}
}

func TestCheckShrinksToMinimal(t *testing.T) {
	g := List(8, IntRange(0, 1000))
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
