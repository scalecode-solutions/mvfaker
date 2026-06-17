package mvfaker_test

import (
	"reflect"
	"strings"
	"testing"

	mvfaker "github.com/scalecode-solutions/mvfaker"
	"github.com/scalecode-solutions/mvfaker/gen"
)

type Address struct {
	City string `fake:"name.last"` // reuse a table; just exercising nesting
	Zip  int    `fake:"number,min=10000,max=99999"`
}

type User struct {
	Name    string `fake:"name.full"`
	Email   string `fake:"internet.email,from=Name"` // coherence
	Age     int    `fake:"number,min=18,max=90"`
	VIP     bool   `fake:"bool,p=0.5"`
	Home    Address
	Aliases []string
	secret  string // unexported: must stay empty
}

func TestFillDeterministicAndCoherent(t *testing.T) {
	var a, b User
	if err := mvfaker.FillAt(&a, 42); err != nil {
		t.Fatal(err)
	}
	if err := mvfaker.FillAt(&b, 42); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("FillAt not deterministic:\n %+v\n %+v", a, b)
	}

	wantLocal := strings.ReplaceAll(strings.ToLower(a.Name), " ", ".") + "@"
	if !strings.HasPrefix(a.Email, wantLocal) {
		t.Fatalf("email %q not coherent with name %q", a.Email, a.Name)
	}
	if a.Age < 18 || a.Age > 90 {
		t.Fatalf("age %d out of tagged range", a.Age)
	}
	if a.Home.Zip < 10000 || a.Home.Zip > 99999 {
		t.Fatalf("nested zip %d out of range", a.Home.Zip)
	}
	if a.secret != "" {
		t.Fatal("unexported field should not be filled")
	}
}

func TestStructComposesWithGenerators(t *testing.T) {
	// A struct generator is just another Generator — usable inside Slice.
	g := gen.Slice(gen.IntRange(2, 2), mvfaker.Struct[User]())
	users := g.Generate(gen.At(7, 1))
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	for _, u := range users {
		if u.Name == "" || u.Email == "" {
			t.Fatalf("composed user not filled: %+v", u)
		}
	}
}

func TestInferenceNoTags(t *testing.T) {
	type Lead struct {
		FullName string // inferred name.full
		Email    string // inferred internet.email, from FullName
		Active   bool   // inferred bool
	}
	var l Lead
	if err := mvfaker.FillAt(&l, 3); err != nil {
		t.Fatal(err)
	}
	if l.FullName == "" {
		t.Fatal("inferred name not filled")
	}
	wantLocal := strings.ReplaceAll(strings.ToLower(l.FullName), " ", ".") + "@"
	if !strings.HasPrefix(l.Email, wantLocal) {
		t.Fatalf("inferred email %q not coherent with %q", l.Email, l.FullName)
	}
}
