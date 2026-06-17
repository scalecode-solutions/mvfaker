package data

import (
	"fmt"
	"strings"

	"github.com/scalecode-solutions/mvfaker/gen"
)

func init() {
	// Public IPv4 — reject the private/reserved blocks so it looks routable.
	Register("ipv4", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			for {
				a, b, c, d := int(s.Draw(256)), int(s.Draw(256)), int(s.Draw(256)), int(s.Draw(256))
				if publicV4(a, b, c, d) {
					return fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
				}
			}
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// Private IPv4 — one of the three RFC 1918 blocks.
	Register("ipv4.private", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			switch s.Draw(3) {
			case 0:
				return fmt.Sprintf("10.%d.%d.%d", s.Draw(256), s.Draw(256), 1+s.Draw(254))
			case 1:
				return fmt.Sprintf("172.%d.%d.%d", 16+s.Draw(16), s.Draw(256), 1+s.Draw(254))
			default:
				return fmt.Sprintf("192.168.%d.%d", s.Draw(256), 1+s.Draw(254))
			}
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// IPv6 — global-unicast (2000::/3), canonical RFC 5952 form: lowercase, no
	// leading zeros, and "::" compressing the longest run of zero hextets.
	Register("ipv6", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string { return ipv6(s) })
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	Register("mac", func(Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
				s.Draw(256), s.Draw(256), s.Draw(256), s.Draw(256), s.Draw(256), s.Draw(256))
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})
}

func publicV4(a, b, c, d int) bool {
	switch {
	case a == 0, a == 10, a == 127, a >= 224: // this-net, RFC1918-10, loopback, multicast/reserved
		return false
	case a == 169 && b == 254: // link-local
		return false
	case a == 172 && b >= 16 && b <= 31: // RFC1918-172
		return false
	case a == 192 && b == 168: // RFC1918-192
		return false
	case a == 255: // broadcast-ish
		return false
	}
	return true
}

func ipv6(s gen.Source) string {
	h := make([]int, 8)
	h[0] = 0x2000 + int(s.Draw(0x2000)) // 2000–3fff: global unicast
	for i := 1; i < 8; i++ {
		if s.Draw(4) == 0 { // ~25% zero hextets — realistic, and exercises "::"
			h[i] = 0
		} else {
			h[i] = int(s.Draw(0x10000))
		}
	}

	// longest run of consecutive zero hextets (>=2), leftmost wins ties
	bestStart, bestLen := -1, 0
	for i := 0; i < 8; {
		if h[i] == 0 {
			j := i
			for j < 8 && h[j] == 0 {
				j++
			}
			if j-i > bestLen {
				bestStart, bestLen = i, j-i
			}
			i = j
		} else {
			i++
		}
	}

	parts := make([]string, 8)
	for i, x := range h {
		parts[i] = fmt.Sprintf("%x", x)
	}
	if bestLen >= 2 {
		return strings.Join(parts[:bestStart], ":") + "::" + strings.Join(parts[bestStart+bestLen:], ":")
	}
	return strings.Join(parts, ":")
}
