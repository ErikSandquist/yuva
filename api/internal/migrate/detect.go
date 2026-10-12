package migrate

import (
	"context"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	FoundByDomain     = "domain"
	FoundByMX         = "mx"
	FoundByAutoconfig = "autoconfig"
	FoundBySRV        = "srv"
	FoundByGuess      = "guess"
	FoundByNone       = "none"

	NoteMicrosoftLater = "microsoft_later"
	NoteProton         = "proton"
	NoteCloudflare     = "cloudflare"
)

type Detection struct {
	Address   string
	Domain    string
	Preset    string
	Name      string
	Available bool
	Note      string
	Server    *Server
	Auth      []string
	Guide     string
	MX        []string
	FoundBy   string
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Detector struct {
	Resolver     Resolver
	HTTP         HTTPDoer
	AllowPrivate bool
	// MicrosoftAvailable is whether the server can offer Sign in with Microsoft.
	MicrosoftAvailable bool
	// Timeout bounds each lookup and connection; 5 seconds when zero.
	Timeout time.Duration
}

var tldish = regexp.MustCompile(`^[a-z]{2,3}(\.[a-z]{2,3})?$`)

// consumer maps a mail service's own domains to its preset and, where the domain decides it,
// the server.
func consumer(domain string) (preset, host string, ok bool) {
	switch domain {
	case "gmail.com", "googlemail.com":
		return PresetGmail, "", true
	case "msn.com":
		return PresetMicrosoft, "", true
	case "ya.ru":
		return PresetYandex, "", true
	case "icloud.com", "me.com", "mac.com":
		return PresetICloud, "", true
	case "zoho.com":
		return PresetZoho, "imap.zoho.com", true
	case "ymail.com", "rocketmail.com":
		return PresetYahoo, "", true
	case "mail.com":
		return PresetGMX, "imap.mail.com", true
	}
	first, rest, found := strings.Cut(domain, ".")
	if !found || !tldish.MatchString(rest) {
		return "", "", false
	}
	switch first {
	case "outlook", "hotmail", "live":
		return PresetMicrosoft, "", true
	case "yandex":
		return PresetYandex, "", true
	case "zohomail":
		if _, ok := (Preset{Servers: tls993(zohoHosts...)}).ServerFor("imap.zoho." + rest); ok {
			return PresetZoho, "imap.zoho." + rest, true
		}
		return PresetZoho, "imap.zoho.com", true
	case "yahoo":
		return PresetYahoo, "", true
	case "fastmail":
		return PresetFastmail, "", true
	case "gmx":
		switch rest {
		case "com", "co.uk", "fr", "es", "us":
			return PresetGMX, "imap.gmx.com", true
		}
		return PresetGMX, "imap.gmx.net", true
	}
	return "", "", false
}

type mxRule struct {
	exact  []string
	suffix []string
	re     *regexp.Regexp
	preset string
	note   string
}

var zohoMX = regexp.MustCompile(`^mx[23]?\.zoho\.(com|eu|in|com\.au|jp|ca|sa)$`)

var mxRules = []mxRule{
	{exact: []string{"smtp.google.com", "aspmx.l.google.com"}, suffix: []string{".aspmx.l.google.com", ".googlemail.com"}, preset: PresetGoogleWorkspace},
	{suffix: []string{".mail.protection.outlook.com", ".mx.microsoft"}, preset: PresetMicrosoft},
	{exact: []string{"mx.yandex.net", "mx.yandex.ru"}, preset: PresetYandex},
	{re: zohoMX, preset: PresetZoho},
	{exact: []string{"mx01.mail.icloud.com", "mx02.mail.icloud.com"}, preset: PresetICloud},
	{suffix: []string{".yahoodns.net"}, preset: PresetYahoo},
	{exact: []string{"in1-smtp.messagingengine.com", "in2-smtp.messagingengine.com"}, preset: PresetFastmail},
	{exact: []string{"mx00.gmx.net", "mx01.gmx.net", "mx00.mail.com"}, preset: PresetGMX},
	{suffix: []string{".mx.cloudflare.net"}, preset: PresetIMAP, note: NoteCloudflare},
	{exact: []string{"mail.protonmail.ch", "mailsec.protonmail.ch"}, preset: PresetIMAP, note: NoteProton},
}

// MatchMX finds the rule of one MX host: its preset, a note, and for Zoho the server of its data
// centre.
func MatchMX(host string) (preset, note, server string, ok bool) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, r := range mxRules {
		hit := slices.Contains(r.exact, host)
		for _, s := range r.suffix {
			hit = hit || strings.HasSuffix(host, s)
		}
		if r.re != nil {
			if m := r.re.FindStringSubmatch(host); m != nil {
				hit, server = true, "imappro.zoho."+m[1]
			}
		}
		if hit {
			return r.preset, r.note, server, true
		}
	}
	return "", "", "", false
}

func (d Detector) timeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return 5 * time.Second
}

// Detect picks a preset for address: consumer domains by name, then the domain's MX, then the
// domain's own autoconfig, its _imaps._tcp SRV record, and imap. or mail. of the domain.
func (d Detector) Detect(ctx context.Context, address string) Detection {
	address = strings.ToLower(strings.TrimSpace(address))
	_, domain, _ := strings.Cut(address, "@")
	domain = strings.TrimSuffix(domain, ".")
	out := Detection{Address: address, Domain: domain, MX: []string{}}
	if id, host, ok := consumer(domain); ok {
		return d.withPreset(out, id, host, "", FoundByDomain)
	}
	mctx, cancel := context.WithTimeout(ctx, d.timeout())
	mxs, _ := d.Resolver.LookupMX(mctx, domain)
	cancel()
	slices.SortStableFunc(mxs, func(a, b *net.MX) int { return int(a.Pref) - int(b.Pref) })
	for _, mx := range mxs {
		out.MX = append(out.MX, strings.TrimSuffix(strings.ToLower(mx.Host), "."))
	}
	for _, mx := range out.MX {
		if id, note, server, ok := MatchMX(mx); ok {
			if note != "" {
				out = d.withPreset(out, id, "", note, FoundByMX)
				if note == NoteProton {
					out.Available = false
				}
				return out
			}
			return d.withPreset(out, id, server, "", FoundByMX)
		}
	}
	out = d.withPreset(out, PresetIMAP, "", "", FoundByNone)
	if s, ok := d.autoconfig(ctx, address, domain); ok {
		out.Server, out.FoundBy = &s, FoundByAutoconfig
		return out
	}
	if s, ok := d.srv(ctx, domain); ok {
		out.Server, out.FoundBy = &s, FoundBySRV
		return out
	}
	for _, h := range []string{"imap." + domain, "mail." + domain} {
		if d.answers(ctx, h, 993) {
			out.Server, out.FoundBy = &Server{Host: h, Port: 993, TLS: "tls"}, FoundByGuess
			return out
		}
	}
	return out
}

func (d Detector) withPreset(out Detection, id, host, note, by string) Detection {
	p, _ := LookupPreset(id)
	out.Preset, out.Name, out.Available, out.Note, out.Auth, out.Guide, out.FoundBy = p.ID, p.Name, p.Available, note, p.Auth, p.Guide, by
	if s, ok := p.ServerFor(host); ok {
		out.Server = &s
	}
	if id == PresetMicrosoft {
		out.Available = d.MicrosoftAvailable
		if !out.Available {
			out.Note = NoteMicrosoftLater
		}
	}
	return out
}

// MicrosoftAccount tells whether address is a personal Microsoft account (Outlook.com, Hotmail,
// Live, MSN) or a work one, whose domain's mail Microsoft 365 receives; neither when unknown.
func MicrosoftAccount(ctx context.Context, r Resolver, address string, timeout time.Duration) (personal, work bool) {
	_, domain, _ := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	domain = strings.TrimSuffix(domain, ".")
	if id, _, ok := consumer(domain); ok {
		return id == PresetMicrosoft, false
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	mxs, _ := r.LookupMX(ctx, domain)
	for _, mx := range mxs {
		if id, _, _, ok := MatchMX(mx.Host); ok && id == PresetMicrosoft {
			return false, true
		}
	}
	return false, false
}

type autoconfigDoc struct {
	Providers []struct {
		Incoming []struct {
			Type       string `xml:"type,attr"`
			Hostname   string `xml:"hostname"`
			Port       int    `xml:"port"`
			SocketType string `xml:"socketType"`
		} `xml:"incomingServer"`
	} `xml:"emailProvider"`
}

func (d Detector) autoconfig(ctx context.Context, address, domain string) (Server, bool) {
	if d.HTTP == nil {
		return Server{}, false
	}
	urls := []string{
		"https://autoconfig." + domain + "/mail/config-v1.1.xml?emailaddress=" + url.QueryEscape(address),
		"https://" + domain + "/.well-known/autoconfig/mail/config-v1.1.xml",
	}
	for _, u := range urls {
		if s, ok := d.fetchAutoconfig(ctx, u, address, domain); ok {
			return s, true
		}
	}
	return Server{}, false
}

func (d Detector) fetchAutoconfig(ctx context.Context, u, address, domain string) (Server, bool) {
	cctx, cancel := context.WithTimeout(ctx, d.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, u, nil)
	if err != nil {
		return Server{}, false
	}
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return Server{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Server{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return Server{}, false
	}
	return parseAutoconfig(body, address, domain)
}

func parseAutoconfig(body []byte, address, domain string) (Server, bool) {
	var doc autoconfigDoc
	if xml.Unmarshal(body, &doc) != nil {
		return Server{}, false
	}
	local, _, _ := strings.Cut(address, "@")
	for _, p := range doc.Providers {
		for _, in := range p.Incoming {
			if !strings.EqualFold(in.Type, "imap") {
				continue
			}
			host := strings.NewReplacer("%EMAILDOMAIN%", domain, "%EMAILLOCALPART%", local, "%EMAILADDRESS%", address).Replace(strings.TrimSpace(in.Hostname))
			if !ValidHost(host) || in.Port < 1 || in.Port > 65535 {
				continue
			}
			switch strings.ToUpper(in.SocketType) {
			case "SSL":
				return Server{Host: strings.ToLower(host), Port: in.Port, TLS: "tls"}, true
			case "STARTTLS":
				return Server{Host: strings.ToLower(host), Port: in.Port, TLS: "starttls"}, true
			}
		}
	}
	return Server{}, false
}

func (d Detector) srv(ctx context.Context, domain string) (Server, bool) {
	cctx, cancel := context.WithTimeout(ctx, d.timeout())
	defer cancel()
	_, recs, err := d.Resolver.LookupSRV(cctx, "imaps", "tcp", domain)
	if err != nil {
		return Server{}, false
	}
	for _, r := range recs {
		host := strings.TrimSuffix(strings.ToLower(r.Target), ".")
		if host == "" || r.Port == 0 || !ValidHost(host) {
			continue
		}
		return Server{Host: host, Port: int(r.Port), TLS: "tls"}, true
	}
	return Server{}, false
}

func (d Detector) answers(ctx context.Context, host string, port int) bool {
	cctx, cancel := context.WithTimeout(ctx, d.timeout())
	defer cancel()
	ips, err := Addresses(cctx, d.Resolver, host, d.AllowPrivate)
	if err != nil {
		return false
	}
	conn, err := DialAddresses(cctx, ips, port, d.timeout())
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
