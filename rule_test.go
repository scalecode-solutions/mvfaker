package mvfaker_test

import (
	"testing"

	mvfaker "github.com/scalecode-solutions/mvfaker"
	"github.com/scalecode-solutions/mvfaker/gen"
)

func TestRegisteredRuleFailsAndShrinks(t *testing.T) {
	mvfaker.RegisterRule("test.no-big",
		gen.List(8, gen.IntRange(0, 1000)),
		func(xs []int) bool {
			for _, x := range xs {
				if x >= 900 {
					return false
				}
			}
			return true
		})
	r, err := mvfaker.RunRule("test.no-big", 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Failed {
		t.Fatal("expected rule to fail")
	}
	if r.Example != "[900]" {
		t.Fatalf("expected shrink to [900], got %s", r.Example)
	}
}

func TestRegisteredRuleHolds(t *testing.T) {
	mvfaker.RegisterRule("test.abs",
		gen.IntRange(-1000, 1000),
		func(x int) bool {
			if x < 0 {
				x = -x
			}
			return x >= 0
		})
	r, _ := mvfaker.RunRule("test.abs", 1, 500)
	if r.Failed {
		t.Fatalf("expected rule to hold, failed at %s", r.Example)
	}
}

func TestUnknownRule(t *testing.T) {
	if _, err := mvfaker.RunRule("nope", 1, 10); err == nil {
		t.Fatal("expected error for unknown rule")
	}
}

// Self-referential type must terminate (cycle guard).
type Node struct {
	Val  int `fake:"number,min=1,max=9"`
	Next *Node
}

func TestCycleGuardTerminates(t *testing.T) {
	// Without the guard this recurses forever and the test times out.
	var n Node
	if err := mvfaker.FillAt(&n, 1); err != nil {
		t.Fatal(err)
	}
	depth := 0
	for p := &n; p != nil; p = p.Next {
		depth++
		if depth > maxDepthForTest {
			t.Fatal("cycle guard failed: chain too deep")
		}
	}
}

const maxDepthForTest = 64

func TestSliceLenTag(t *testing.T) {
	type Bag struct {
		Items []int `fake:"len=5"`
	}
	var b Bag
	if err := mvfaker.FillAt(&b, 2); err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 5 {
		t.Fatalf("expected len=5, got %d", len(b.Items))
	}
}
