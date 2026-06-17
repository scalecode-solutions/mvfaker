package data

import (
	"net"
	"testing"

	"github.com/scalecode-solutions/mvfaker/gen"
)

func TestIPv4Public(t *testing.T) {
	mk, _ := Build("ipv4", nil)
	g := mk(nil)
	for i := 0; i < 300; i++ {
		s := g.Generate(gen.At(uint64(i))).(string)
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() == nil {
			t.Fatalf("%q is not a valid IPv4", s)
		}
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
			t.Fatalf("%q is in a reserved/private range", s)
		}
	}
}

func TestIPv4Private(t *testing.T) {
	mk, _ := Build("ipv4.private", nil)
	g := mk(nil)
	for i := 0; i < 100; i++ {
		s := g.Generate(gen.At(uint64(i))).(string)
		ip := net.ParseIP(s)
		if ip == nil || !ip.IsPrivate() {
			t.Fatalf("%q should be private", s)
		}
	}
}

func TestIPv6Valid(t *testing.T) {
	mk, _ := Build("ipv6", nil)
	g := mk(nil)
	sawCompressed := false
	for i := 0; i < 300; i++ {
		s := g.Generate(gen.At(uint64(i))).(string)
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() != nil { // must parse, and be v6 (not v4)
			t.Fatalf("%q is not a valid IPv6", s)
		}
		if len(s) >= 2 && containsCompression(s) {
			sawCompressed = true
		}
	}
	if !sawCompressed {
		t.Fatal("expected some addresses to use :: compression")
	}
}

func TestMAC(t *testing.T) {
	mk, _ := Build("mac", nil)
	g := mk(nil)
	for i := 0; i < 100; i++ {
		s := g.Generate(gen.At(uint64(i))).(string)
		if _, err := net.ParseMAC(s); err != nil {
			t.Fatalf("%q is not a valid MAC: %v", s, err)
		}
	}
}

func containsCompression(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == ':' && s[i+1] == ':' {
			return true
		}
	}
	return false
}
