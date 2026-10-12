package analytics

import (
	"net/netip"
)

// ShortIP is a client address cut to its /24 (IPv4) or /48 (IPv6) network. It is the only form of
// an address that hashing, rate limits and location lookups accept, so the full address never
// reaches them.
type ShortIP struct {
	p netip.Prefix
}

// Shorten parses addr and drops everything but its network; ok is false for anything that is not
// an IP address.
func Shorten(addr string) (ShortIP, bool) {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return ShortIP{}, false
	}
	return ShortenAddr(a), true
}

func ShortenAddr(a netip.Addr) ShortIP {
	a = a.Unmap().WithZone("")
	bits := 48
	if a.Is4() {
		bits = 24
	}
	return ShortIP{p: netip.PrefixFrom(a, bits).Masked()}
}

func (s ShortIP) String() string { return s.p.String() }

// Addr is the network's first address, for location lookups.
func (s ShortIP) Addr() netip.Addr { return s.p.Addr() }

func (s ShortIP) key() []byte {
	b := s.p.Addr().As16()
	out := make([]byte, 0, 17)
	if s.p.Addr().Is4() {
		out = append(out, 4)
	} else {
		out = append(out, 6)
	}
	return append(out, b[:]...)
}
