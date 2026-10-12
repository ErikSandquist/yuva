package migrate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/productdevbook/yuva/api/internal/webhook"
)

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

var (
	ErrNoAddress  = errors.New("the name has no address")
	ErrNotAllowed = errors.New("the server is on a private or local network")
)

// Addresses resolves host once and refuses it when any address is link-local or a cloud metadata
// address, or private, loopback or otherwise not public unless allowPrivate, as for SMTP hosts
// and webhooks.
func Addresses(ctx context.Context, r Resolver, host string, allowPrivate bool) ([]netip.Addr, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		ips = []netip.Addr{ip}
	} else {
		if !allowPrivate && (host == "localhost" || strings.HasSuffix(host, ".localhost")) {
			return nil, ErrNotAllowed
		}
		if ips, err = r.LookupNetIP(ctx, "ip", host); err != nil {
			return nil, err
		}
	}
	if len(ips) == 0 {
		return nil, ErrNoAddress
	}
	for _, ip := range ips {
		if webhook.NeverAllowed(ip) || (!allowPrivate && webhook.Blocked(ip)) {
			return nil, fmt.Errorf("%w (%s)", ErrNotAllowed, ip.Unmap())
		}
	}
	return ips, nil
}

// DialAddresses connects to the first of ips that answers on port.
func DialAddresses(ctx context.Context, ips []netip.Addr, port int, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	var last error
	for _, ip := range ips {
		conn, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(ip.Unmap(), uint16(port)).String())
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// Dialer connects only to addresses Addresses allows.
func Dialer(r Resolver, allowPrivate bool, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		ips, err := Addresses(ctx, r, host, allowPrivate)
		if err != nil {
			return nil, err
		}
		return DialAddresses(ctx, ips, p, timeout)
	}
}

// ValidHost accepts a DNS name or an IP address.
func ValidHost(h string) bool {
	h = strings.TrimSuffix(h, ".")
	if _, err := netip.ParseAddr(h); err == nil {
		return true
	}
	if h == "" || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return false
			}
		}
	}
	return true
}
