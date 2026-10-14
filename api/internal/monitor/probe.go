package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/productdevbook/yuva/api/internal/webhook"
)

// ProbeBodyBytes is how much of an answer a check reads.
const ProbeBodyBytes = 64 << 10

// Why a run failed.
const (
	ProbeDNS            = "dns"
	ProbeRefusedAddress = "refused_address"
	ProbeConnect        = "connect"
	ProbeTLS            = "tls"
	ProbeTimeout        = "timeout"
	ProbeStatus         = "status"
	ProbeKeyword        = "keyword"
	ProbeRedirects      = "redirects"
	ProbeOther          = "other"
)

// ProbeSpec is one run of an uptime check.
type ProbeSpec struct {
	URL           string
	Method        string
	Headers       [][2]string
	Expected      []int
	Keyword       string
	KeywordAbsent bool
	Timeout       time.Duration
	MaxRedirects  int
	UserAgent     string
	// AnyStatus takes every status as an answer and KeepBody keeps what was read, for health pulls
	// that judge the body.
	AnyStatus bool
	KeepBody  bool
	Accept    string
}

// ProbeResult is what a run found. Timings add up the redirect hops; TTFB is until the first byte of
// the last answer and Total until its body was read.
type ProbeResult struct {
	OK          bool
	Status      int
	Error       string
	Detail      string
	Redirects   int
	Answered    bool
	DNS         time.Duration
	Connect     time.Duration
	TLS         time.Duration
	TTFB        time.Duration
	Total       time.Duration
	CertExpires *time.Time
	Body        []byte
	ContentType string
}

// Prober calls check URLs through the webhook client's dialer: each connection resolves its host
// once, refuses loopback, private, link-local, CGNAT, multicast and reserved addresses (link-local
// and metadata ones even with AllowPrivate) and connects to the resolved address only.
type Prober struct {
	AllowPrivate bool
	hooks        *webhook.Client
	rootCAs      *x509.CertPool
}

func NewProber(allowPrivate bool, r webhook.Resolver, connect func(ctx context.Context, network, address string) (net.Conn, error)) *Prober {
	if r == nil {
		r = net.DefaultResolver
	}
	hooks := webhook.NewClient(allowPrivate, timedResolver{r})
	hooks.Connect = connect
	return &Prober{AllowPrivate: allowPrivate, hooks: hooks}
}

type timings struct {
	mu                  sync.Mutex
	dns, dial, tls      time.Duration
	tlsStart, firstByte time.Time
	tlsStarted          bool
}

type timingsKey struct{}

type timedResolver struct{ r webhook.Resolver }

func (t timedResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	start := time.Now()
	ips, err := t.r.LookupNetIP(ctx, network, host)
	if tm, ok := ctx.Value(timingsKey{}).(*timings); ok {
		tm.mu.Lock()
		tm.dns += time.Since(start)
		tm.mu.Unlock()
	}
	return ips, err
}

var (
	errTooManyRedirects = errors.New("too many redirects")
	errRedirectRefused  = errors.New("redirect refused")
)

// CheckTarget refuses a check URL that names credentials, is not http or https, or names
// localhost or an address the server will not call.
func (p *Prober) CheckTarget(raw string) (*url.URL, error) {
	return webhook.CheckURL(raw, p.AllowPrivate)
}

// Refused reports whether ip is an address checks may not reach.
func (p *Prober) Refused(ip netip.Addr) bool {
	return webhook.NeverAllowed(ip) || (!p.AllowPrivate && webhook.Blocked(ip))
}

func (p *Prober) Probe(ctx context.Context, spec ProbeSpec) ProbeResult {
	var res ProbeResult
	u, err := p.CheckTarget(spec.URL)
	if err != nil {
		return failed(res, classify(err, false), "")
	}
	tm := &timings{}
	ctx, cancel := context.WithTimeout(context.WithValue(ctx, timingsKey{}, tm), spec.Timeout)
	defer cancel()
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() {
			tm.mu.Lock()
			tm.tlsStart, tm.tlsStarted = time.Now(), true
			tm.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			tm.mu.Lock()
			tm.tls += time.Since(tm.tlsStart)
			tm.tlsStarted = false
			tm.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			tm.mu.Lock()
			tm.firstByte = time.Now()
			tm.mu.Unlock()
		},
	}
	ctx = httptrace.WithClientTrace(ctx, trace)
	names := make([]string, 0, len(spec.Headers))
	for _, h := range spec.Headers {
		names = append(names, h[0])
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				tm.mu.Lock()
				before := tm.dns
				tm.mu.Unlock()
				start := time.Now()
				conn, err := p.hooks.DialContext(ctx, network, addr)
				tm.mu.Lock()
				tm.dial += time.Since(start) - (tm.dns - before)
				tm.mu.Unlock()
				return conn, err
			},
			DisableKeepAlives:      true,
			TLSClientConfig:        &tls.Config{RootCAs: p.rootCAs},
			TLSHandshakeTimeout:    spec.Timeout,
			ResponseHeaderTimeout:  spec.Timeout,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if spec.MaxRedirects == 0 {
				return http.ErrUseLastResponse
			}
			if len(via) > spec.MaxRedirects {
				return errTooManyRedirects
			}
			if _, err := p.CheckTarget(req.URL.String()); err != nil {
				if errors.Is(err, webhook.ErrRefusedAddress) {
					return err
				}
				return errRedirectRefused
			}
			if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				for _, n := range names {
					req.Header.Del(n)
				}
			}
			res.Redirects = len(via)
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, spec.Method, u.String(), nil)
	if err != nil {
		return failed(res, ProbeOther, "the request could not be made")
	}
	for _, h := range spec.Headers {
		req.Header.Set(h[0], h[1])
	}
	req.Header.Set("User-Agent", spec.UserAgent)
	if spec.Accept != "" {
		req.Header.Set("Accept", spec.Accept)
	}
	start := time.Now()
	resp, err := client.Do(req)
	finish := func() {
		tm.mu.Lock()
		defer tm.mu.Unlock()
		res.DNS, res.Connect, res.TLS, res.Total = tm.dns, max(tm.dial, 0), tm.tls, time.Since(start)
		if !tm.firstByte.IsZero() {
			res.TTFB = tm.firstByte.Sub(start)
		}
	}
	if err != nil {
		finish()
		tm.mu.Lock()
		inTLS := tm.tlsStarted
		tm.mu.Unlock()
		return failed(res, classify(err, inTLS), "")
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, ProbeBodyBytes))
	finish()
	res.Status, res.Answered = resp.StatusCode, true
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		res.CertExpires = new(resp.TLS.PeerCertificates[0].NotAfter.UTC())
	}
	if spec.KeepBody {
		if readErr != nil {
			return failed(res, classify(readErr, false), "")
		}
		res.Body, res.ContentType = body, resp.Header.Get("Content-Type")
	}
	if !spec.AnyStatus && !expected(spec.Expected, resp.StatusCode) {
		return failed(res, ProbeStatus, "answered "+strconv.Itoa(resp.StatusCode))
	}
	if spec.Keyword != "" && spec.Method == http.MethodGet {
		if readErr != nil {
			return failed(res, classify(readErr, false), "")
		}
		if bytes.Contains(body, []byte(spec.Keyword)) == spec.KeywordAbsent {
			if spec.KeywordAbsent {
				return failed(res, ProbeKeyword, "the answer contains the keyword")
			}
			return failed(res, ProbeKeyword, "the answer does not contain the keyword")
		}
	}
	res.OK = true
	return res
}

func expected(list []int, status int) bool {
	if len(list) == 0 {
		return status >= 200 && status <= 399
	}
	return slices.Contains(list, status)
}

var probeDetails = map[string]string{
	ProbeDNS:            "the host name did not resolve",
	ProbeRefusedAddress: "the host resolves, or a redirect leads, to an address the server does not call",
	ProbeConnect:        "the connection failed",
	ProbeTLS:            "the TLS handshake or the certificate failed",
	ProbeTimeout:        "no answer within the timeout",
	ProbeRedirects:      "too many redirects, or a redirect to a URL that is not allowed",
	ProbeOther:          "the request failed",
}

// failed fills in a fixed detail per reason, so a run never tells members what a host resolved to.
func failed(res ProbeResult, reason, detail string) ProbeResult {
	res.OK, res.Error = false, reason
	if detail == "" {
		detail = probeDetails[reason]
	}
	res.Detail = detail
	return res
}

func classify(err error, inTLS bool) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	var certErr *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	var recordErr tls.RecordHeaderError
	var opErr *net.OpError
	switch {
	case errors.Is(err, webhook.ErrRefusedAddress):
		return ProbeRefusedAddress
	case errors.Is(err, errTooManyRedirects), errors.Is(err, errRedirectRefused):
		return ProbeRedirects
	case errors.As(err, &dnsErr), strings.Contains(err.Error(), "has no address"):
		return ProbeDNS
	case errors.As(err, &certErr), errors.As(err, &unknownCA), errors.As(err, &hostErr), errors.As(err, &invalid), errors.As(err, &recordErr):
		return ProbeTLS
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return ProbeTimeout
	case inTLS:
		return ProbeTLS
	case errors.As(err, &opErr) && opErr.Op == "dial":
		return ProbeConnect
	}
	return ProbeOther
}
