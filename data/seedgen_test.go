package data

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scalecode-solutions/mvfaker/gen"
	"golang.org/x/crypto/bcrypt"
)

func TestOneofWeighted(t *testing.T) {
	mk, err := Build("oneof", Params{"values": []any{"a", "b", "c"}, "weights": []any{8, 1, 1}})
	if err != nil {
		t.Fatal(err)
	}
	g := mk(nil)
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		counts[g.Generate(gen.At(uint64(i))).(string)]++
	}
	if counts["a"] < 600 {
		t.Fatalf("weighted oneof: 'a' should dominate, got %v", counts)
	}
}

func TestOneofRequiresValues(t *testing.T) {
	if _, err := Build("oneof", nil); err == nil {
		t.Fatal("expected error when values is missing")
	}
}

func TestTimestampRange(t *testing.T) {
	now := time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC)
	mk, _ := Build("timestamp", Params{"days_ago_min": 0, "days_ago_max": 10, "now": "2026-01-11T00:00:00Z"})
	g := mk(nil)
	for i := 0; i < 100; i++ {
		ts := g.Generate(gen.At(uint64(i))).(string)
		tm, err := time.Parse("2006-01-02 15:04:05", ts)
		if err != nil {
			t.Fatalf("bad timestamp %q: %v", ts, err)
		}
		if tm.After(now) || tm.Before(now.AddDate(0, 0, -11)) {
			t.Fatalf("timestamp %v outside [now-11d, now]", tm)
		}
	}
}

func TestJSONGenValid(t *testing.T) {
	mk, _ := Build("json", nil)
	g := mk(nil)
	for i := 0; i < 50; i++ {
		var m map[string]any
		if err := json.Unmarshal([]byte(g.Generate(gen.At(uint64(i))).(string)), &m); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
	}
}

func TestBcryptVerifies(t *testing.T) {
	mk, _ := Build("password.bcrypt", Params{"plaintext": "hunter2", "cost": 4}) // low cost = fast test
	h := mk(nil).Generate(gen.At(1)).(string)
	if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("hunter2")); err != nil {
		t.Fatalf("hash does not verify against plaintext: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(h), []byte("wrong")) == nil {
		t.Fatal("hash verified against the wrong password")
	}
}

func TestCopyReturnsDep(t *testing.T) {
	mk, _ := Build("copy", nil)
	if got := mk("Hello").Generate(gen.At(1)); got != "Hello" {
		t.Fatalf("copy returned %v, want Hello", got)
	}
}
