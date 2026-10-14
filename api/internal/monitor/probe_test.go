package monitor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// publicAddr stands for a public address in these tests: connections to it go to the local test
// server instead, so nothing outside is ever called.
const publicAddr = "93.184.216.34"

type answers struct {
	mu    sync.Mutex
	calls map[string]int
	seq   map[string][][]netip.Addr
}

func (a *answers) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := a.calls[host]
	a.calls[host] = n + 1
	list := a.seq[host]
	if len(list) == 0 {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return list[min(n, len(list)-1)], nil
}

func addrs(v ...string) []netip.Addr {
	out := make([]netip.Addr, len(v))
	for i, s := range v {
		out[i] = netip.MustParseAddr(s)
	}
	return out
}

type probeEnv struct {
	srv     *httptest.Server
	port    string
	hits    atomic.Int32
	dialed  []string
	mu      sync.Mutex
	answers *answers
}

func newProbeEnv(t *testing.T, h http.HandlerFunc) *probeEnv {
	t.Helper()
	e := &probeEnv{answers: &answers{calls: map[string]int{}, seq: map[string][][]netip.Addr{}}}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(e.srv.Close)
	e.port = e.srv.URL[strings.LastIndex(e.srv.URL, ":")+1:]
	return e
}

func (e *probeEnv) prober(allowPrivate bool) *Prober {
	local := e.srv.Listener.Addr().String()
	return NewProber(allowPrivate, e.answers, func(ctx context.Context, network, address string) (net.Conn, error) {
		e.mu.Lock()
		e.dialed = append(e.dialed, address)
		e.mu.Unlock()
		if strings.HasPrefix(address, publicAddr+":") {
			address = local
		}
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	})
}

func spec(url string) ProbeSpec {
	return ProbeSpec{URL: url, Method: http.MethodGet, Timeout: 5 * time.Second, MaxRedirects: 5, UserAgent: "Yuva-Uptime/test"}
}

func TestProbeRefusesAddressesTheServerDoesNotCall(t *testing.T) {
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {})
	e.answers.seq["loop.example"] = [][]netip.Addr{addrs("127.0.0.1")}
	e.answers.seq["private.example"] = [][]netip.Addr{addrs("10.1.2.3")}
	e.answers.seq["cgnat.example"] = [][]netip.Addr{addrs("100.64.1.1")}
	e.answers.seq["metadata.example"] = [][]netip.Addr{addrs("169.254.169.254")}
	e.answers.seq["v6loop.example"] = [][]netip.Addr{addrs("::1")}
	e.answers.seq["v6ula.example"] = [][]netip.Addr{addrs("fd00::5")}
	e.answers.seq["v6ll.example"] = [][]netip.Addr{addrs("fe80::1")}
	e.answers.seq["mapped.example"] = [][]netip.Addr{addrs("::ffff:127.0.0.1")}
	e.answers.seq["mappedmeta.example"] = [][]netip.Addr{addrs("::ffff:169.254.169.254")}
	e.answers.seq["mixed.example"] = [][]netip.Addr{addrs(publicAddr, "192.168.1.1")}
	e.answers.seq["multicast.example"] = [][]netip.Addr{addrs("224.0.0.1")}
	p := e.prober(false)
	port := ":" + e.port
	for _, u := range []string{
		e.srv.URL, "http://127.0.0.1" + port, "http://[::1]" + port, "http://10.0.0.1" + port, "http://169.254.169.254/latest/meta-data/",
		"http://[fe80::1]" + port, "http://[::ffff:127.0.0.1]" + port, "http://[::ffff:a9fe:a9fe]/", "http://[fd00:ec2::254]/", "http://localhost" + port,
		"http://loop.example" + port, "http://private.example" + port, "http://cgnat.example" + port, "http://metadata.example" + port,
		"http://v6loop.example" + port, "http://v6ula.example" + port, "http://v6ll.example" + port, "http://mapped.example" + port,
		"http://mappedmeta.example" + port, "http://mixed.example" + port, "http://multicast.example" + port,
	} {
		res := p.Probe(context.Background(), spec(u))
		if res.OK || res.Error != ProbeRefusedAddress {
			t.Errorf("%s: %+v", u, res)
		}
		if strings.Contains(res.Detail, "127.0.0.1") || strings.Contains(res.Detail, "10.") {
			t.Errorf("%s: the detail tells an address: %q", u, res.Detail)
		}
	}
	if e.hits.Load() != 0 {
		t.Fatalf("a refused address was reached %d times", e.hits.Load())
	}
	for _, d := range e.dialed {
		if !strings.HasPrefix(d, publicAddr+":") {
			t.Fatalf("dialed %s", d)
		}
	}
}

func TestProbeRefusesCredentialsAndOtherSchemes(t *testing.T) {
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {})
	p := e.prober(true)
	for _, u := range []string{"http://user:pw@" + e.srv.Listener.Addr().String() + "/", "ftp://example.com/", "file:///etc/passwd", "http:///x", "gopher://x/"} {
		if res := p.Probe(context.Background(), spec(u)); res.OK || res.Error != ProbeOther {
			t.Errorf("%s: %+v", u, res)
		}
	}
	if e.hits.Load() != 0 {
		t.Fatal("a refused URL was called")
	}
}

func TestProbeAllowPrivateStillRefusesMetadata(t *testing.T) {
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {})
	e.answers.seq["metadata.example"] = [][]netip.Addr{addrs("169.254.169.254")}
	e.answers.seq["ll6.example"] = [][]netip.Addr{addrs("fe80::1")}
	e.answers.seq["loop.example"] = [][]netip.Addr{addrs("127.0.0.1")}
	p := e.prober(true)
	port := ":" + e.port
	for _, u := range []string{"http://169.254.169.254" + port, "http://metadata.example" + port, "http://ll6.example" + port, "http://[::ffff:169.254.169.254]" + port,
		"http://[fd00:ec2::254]" + port} {
		if res := p.Probe(context.Background(), spec(u)); res.OK || res.Error != ProbeRefusedAddress {
			t.Errorf("%s: %+v", u, res)
		}
	}
	if e.hits.Load() != 0 {
		t.Fatal("a metadata address was reached")
	}
	if res := p.Probe(context.Background(), spec("http://loop.example"+port+"/")); !res.OK {
		t.Fatalf("loopback with private allowed: %+v", res)
	}
}

// A name that resolves to a public address for the check and to a private one afterwards is
// resolved once per connection and connected to by the address that was checked.
func TestProbeDNSRebinding(t *testing.T) {
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	e.answers.seq["rebind.example"] = [][]netip.Addr{addrs(publicAddr), addrs("127.0.0.1")}
	p := e.prober(false)
	res := p.Probe(context.Background(), spec("http://rebind.example:"+e.port+"/start"))
	if res.OK || res.Error != ProbeRefusedAddress {
		t.Fatalf("rebinding: %+v", res)
	}
	if e.hits.Load() != 1 {
		t.Fatalf("hits %d, want only the first hop", e.hits.Load())
	}
	if len(e.dialed) != 1 || !strings.HasPrefix(e.dialed[0], publicAddr+":") {
		t.Fatalf("dialed %v", e.dialed)
	}
	if e.answers.calls["rebind.example"] != 2 {
		t.Fatalf("lookups %d, want one per connection", e.answers.calls["rebind.example"])
	}
}

func TestProbeRedirectsAreCheckedAgain(t *testing.T) {
	var target, echoed string
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to":
			http.Redirect(w, r, target, http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/echo":
			echoed = r.Header.Get("X-Token")
		}
	})
	e.answers.seq["pub.example"] = [][]netip.Addr{addrs(publicAddr)}
	e.answers.seq["inner.example"] = [][]netip.Addr{addrs("10.0.0.7")}
	p := e.prober(false)
	port := e.port
	for _, to := range []string{
		"http://127.0.0.1:" + port + "/", "http://[::1]:" + port + "/", "http://169.254.169.254/latest/meta-data/", "http://[::ffff:10.0.0.1]:" + port + "/",
		"http://inner.example:" + port + "/", "http://localhost:" + port + "/",
	} {
		target = to
		before := e.hits.Load()
		res := p.Probe(context.Background(), spec("http://pub.example:"+port+"/to"))
		if res.OK || res.Error != ProbeRefusedAddress {
			t.Errorf("redirect to %s: %+v", to, res)
		}
		if e.hits.Load() != before+1 {
			t.Errorf("redirect to %s reached the target", to)
		}
	}
	for _, to := range []string{"http://user:pw@pub.example:" + port + "/", "ftp://pub.example/"} {
		target = to
		if res := p.Probe(context.Background(), spec("http://pub.example:"+port+"/to")); res.OK || res.Error != ProbeRedirects {
			t.Errorf("redirect to %s: %+v", to, res)
		}
	}
	if res := p.Probe(context.Background(), spec("http://pub.example:"+port+"/loop")); res.OK || res.Error != ProbeRedirects || res.Redirects != 5 {
		t.Errorf("redirect loop: %+v", res)
	}
	target = "http://pub.example:" + port + "/fine"
	if res := p.Probe(context.Background(), spec("http://pub.example:"+port+"/to")); !res.OK || res.Redirects != 1 {
		t.Errorf("allowed redirect: %+v", res)
	}
	e.answers.seq["other.example"] = [][]netip.Addr{addrs(publicAddr)}
	for host, want := range map[string]string{"pub.example": "kept", "other.example": ""} {
		target, echoed = "http://"+host+":"+port+"/echo", "unset"
		sp := spec("http://pub.example:" + port + "/to")
		sp.Headers = [][2]string{{"X-Token", "kept"}}
		if res := p.Probe(context.Background(), sp); !res.OK || echoed != want {
			t.Errorf("redirect to %s: %+v, header %q", host, res, echoed)
		}
	}
	none := spec("http://pub.example:" + port + "/to")
	none.MaxRedirects, none.Expected = 0, []int{200}
	if res := p.Probe(context.Background(), none); res.OK || res.Status != http.StatusFound || res.Error != ProbeStatus {
		t.Errorf("redirect not followed: %+v", res)
	}
	none.Expected = []int{302}
	if res := p.Probe(context.Background(), none); !res.OK {
		t.Errorf("redirect expected: %+v", res)
	}
}

func TestProbeStatusKeywordTimeoutAndHeaders(t *testing.T) {
	var gotUA, gotToken, gotCookie string
	e := newProbeEnv(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA, gotToken, gotCookie = r.UserAgent(), r.Header.Get("X-Token"), r.Header.Get("Cookie")
		switch r.URL.Path {
		case "/down":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/slow":
			time.Sleep(1500 * time.Millisecond)
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("a", ProbeBodyBytes) + "needle"))
		default:
			_, _ = w.Write([]byte("status: healthy"))
		}
	})
	e.answers.seq["pub.example"] = [][]netip.Addr{addrs(publicAddr)}
	p := e.prober(false)
	base := "http://pub.example:" + e.port
	sp := spec(base + "/ok")
	sp.Headers = [][2]string{{"X-Token", "secret-value"}}
	sp.Keyword = "healthy"
	res := p.Probe(context.Background(), sp)
	if !res.OK || res.Status != 200 || !res.Answered || gotUA != "Yuva-Uptime/test" || gotToken != "secret-value" || gotCookie != "" {
		t.Fatalf("ok: %+v ua %q token %q", res, gotUA, gotToken)
	}
	if res.Total <= 0 || res.TTFB <= 0 || res.TTFB > res.Total || res.Connect <= 0 {
		t.Fatalf("timings %+v", res)
	}
	sp.KeywordAbsent = true
	if res := p.Probe(context.Background(), sp); res.OK || res.Error != ProbeKeyword {
		t.Fatalf("keyword absent: %+v", res)
	}
	big := spec(base + "/big")
	big.Keyword = "needle"
	if res := p.Probe(context.Background(), big); res.OK || res.Error != ProbeKeyword {
		t.Fatalf("keyword past 64 KiB: %+v", res)
	}
	if res := p.Probe(context.Background(), spec(base+"/down")); res.OK || res.Error != ProbeStatus || res.Status != 503 {
		t.Fatalf("down: %+v", res)
	}
	slow := spec(base + "/slow")
	slow.Timeout = 300 * time.Millisecond
	if res := p.Probe(context.Background(), slow); res.OK || res.Error != ProbeTimeout {
		t.Fatalf("slow: %+v", res)
	}
	if res := p.Probe(context.Background(), spec("http://nowhere.example:"+e.port+"/")); res.OK || res.Error != ProbeDNS {
		t.Fatalf("dns: %+v", res)
	}
}

func TestProbeCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := NewProber(true, nil, nil)
	res := p.Probe(context.Background(), spec(srv.URL))
	if res.OK || res.Error != ProbeTLS {
		t.Fatalf("an untrusted certificate: %+v", res)
	}
	pool := srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	tp := NewProber(true, nil, nil)
	tp.rootCAs = pool
	res = tp.Probe(context.Background(), spec(srv.URL))
	if !res.OK || res.CertExpires == nil || res.TLS <= 0 {
		t.Fatalf("trusted: %+v", res)
	}
	if !res.CertExpires.Equal(srv.Certificate().NotAfter.UTC()) {
		t.Fatalf("expiry %s, want %s", res.CertExpires, srv.Certificate().NotAfter)
	}
}
