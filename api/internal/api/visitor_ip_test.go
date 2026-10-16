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
		{"no forwarded header", "", proxies, "10.0.0.1:4000", nil, "10.0.0.1"},
		{"direct client with a forged header", "", nil, "198.51.100.50:4000", []string{"203.0.113.7"}, "198.51.100.50"},
		{"direct client with a forged chain", "", nil, "198.51.100.50:4000", []string{"192.0.2.1, 203.0.113.7"}, "198.51.100.50"},
		{"direct client, header configured, no proxies", "X-Forwarded-For", nil, "198.51.100.50:4000", []string{"203.0.113.7"}, "198.51.100.50"},
		{"direct client outside the trusted proxies", "X-Forwarded-For", proxies, "198.51.100.50:4000", []string{"203.0.113.7"}, "198.51.100.50"},
		{"trusted proxy", "", proxies, "10.0.0.1:4000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"trusted proxy, client forged an entry", "", proxies, "10.0.0.1:4000", []string{"192.0.2.1, 203.0.113.7"}, "203.0.113.7"},
		{"trusted proxies in a chain", "", proxies, "10.0.0.1:4000", []string{"192.0.2.1, 203.0.113.7, 10.1.2.3"}, "203.0.113.7"},
		{"several header lines", "", proxies, "10.0.0.1:4000", []string{"192.0.2.1", "203.0.113.7", "10.1.2.3"}, "203.0.113.7"},
		{"ipv6 visitor", "", proxies, "10.0.0.1:4000", []string{"2001:db8::5"}, "2001:db8::5"},
		{"mapped visitor", "", proxies, "10.0.0.1:4000", []string{"::ffff:203.0.113.7"}, "203.0.113.7"},
		{"only proxies in the chain", "", proxies, "10.0.0.1:4000", []string{"10.9.9.9, 10.1.2.3"}, "10.0.0.1"},
		{"private client", "", proxies, "10.0.0.1:4000", []string{"192.168.1.4"}, "10.0.0.1"},
		{"loopback", "", proxies, "10.0.0.1:4000", []string{"127.0.0.1"}, "10.0.0.1"},
		{"link local", "", proxies, "10.0.0.1:4000", []string{"fe80::1"}, "10.0.0.1"},
		{"shared address space", "", proxies, "10.0.0.1:4000", []string{"100.64.1.1"}, "10.0.0.1"},
		{"unique local", "", proxies, "10.0.0.1:4000", []string{"fd00::1"}, "10.0.0.1"},
		{"multicast", "", proxies, "10.0.0.1:4000", []string{"224.0.0.1"}, "10.0.0.1"},
		{"malformed", "", proxies, "10.0.0.1:4000", []string{"not-an-address"}, "10.0.0.1"},
		{"with a port", "", proxies, "10.0.0.1:4000", []string{"203.0.113.7:5000"}, "10.0.0.1"},
		{"zone", "", proxies, "10.0.0.1:4000", []string{"2001:db8::5%eth0"}, "10.0.0.1"},
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
