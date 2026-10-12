package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestVisitorIP(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name    string
		header  string
		trusted []netip.Prefix
		peer    string
		xff     []string
		want    string
	}{
		{"no forwarded header", "", nil, "198.51.100.50:4000", nil, "198.51.100.50"},
		{"site proxy, no setting", "", nil, "198.51.100.50:4000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"site proxy chain, no setting", "", nil, "198.51.100.50:4000", []string{"192.0.2.1, 203.0.113.7"}, "192.0.2.1"},
		{"left of the connecting address", "X-Forwarded-For", nil, "10.0.0.1:4000", []string{"203.0.113.7, 198.51.100.50"}, "203.0.113.7"},
		{"behind trusted proxies", "X-Forwarded-For", proxies, "10.0.0.1:4000", []string{"203.0.113.7, 198.51.100.50, 10.1.2.3"}, "203.0.113.7"},
		{"several header lines", "X-Forwarded-For", proxies, "10.0.0.1:4000", []string{"203.0.113.7", "198.51.100.50", "10.1.2.3"}, "203.0.113.7"},
		{"ipv6 visitor", "X-Forwarded-For", nil, "10.0.0.1:4000", []string{"2001:db8::5, 198.51.100.50"}, "2001:db8::5"},
		{"mapped visitor", "", nil, "198.51.100.50:4000", []string{"::ffff:203.0.113.7"}, "203.0.113.7"},
		{"connecting address is the leftmost", "X-Forwarded-For", nil, "10.0.0.1:4000", []string{"198.51.100.50"}, "198.51.100.50"},
		{"private", "", nil, "198.51.100.50:4000", []string{"192.168.1.4"}, "198.51.100.50"},
		{"private in chain", "X-Forwarded-For", nil, "10.0.0.1:4000", []string{"10.9.9.9, 198.51.100.50"}, "198.51.100.50"},
		{"loopback", "", nil, "198.51.100.50:4000", []string{"127.0.0.1"}, "198.51.100.50"},
		{"link local", "", nil, "198.51.100.50:4000", []string{"fe80::1"}, "198.51.100.50"},
		{"shared address space", "", nil, "198.51.100.50:4000", []string{"100.64.1.1"}, "198.51.100.50"},
		{"unique local", "", nil, "198.51.100.50:4000", []string{"fd00::1"}, "198.51.100.50"},
		{"multicast", "", nil, "198.51.100.50:4000", []string{"224.0.0.1"}, "198.51.100.50"},
		{"malformed", "", nil, "198.51.100.50:4000", []string{"not-an-address"}, "198.51.100.50"},
		{"with a port", "", nil, "198.51.100.50:4000", []string{"203.0.113.7:5000"}, "198.51.100.50"},
		{"zone", "", nil, "198.51.100.50:4000", []string{"2001:db8::5%eth0"}, "198.51.100.50"},
	}
	for _, tc := range cases {
		s := &Server{auth: AuthSettings{ClientIPHeader: tc.header, TrustedProxies: tc.trusted}}
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = tc.peer
		for _, v := range tc.xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := s.visitorIP(r); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}
