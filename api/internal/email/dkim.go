package email

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dkim"
	"golang.org/x/net/publicsuffix"
)

// ProbeHeader names the deliverability probe a mail is, next to its Message-ID token.
const ProbeHeader = "X-Yuva-Probe"

// DKIMHeaders are the header fields Yuva's DKIM signature covers. Fields a mail lacks are still
// listed, so nobody can add them later without breaking the signature. Message-ID and Feedback-ID
// are left out because relays such as Amazon SES replace them.
var DKIMHeaders = []string{
	"From", "To", "Subject", "Date", "Reply-To", "MIME-Version", "Content-Type",
	"List-Unsubscribe", "List-Unsubscribe-Post", "List-Id",
}

const dkimKeyBits = 2048

type DKIMKey struct {
	Domain   string
	Selector string
	Signer   crypto.Signer
}

// NewDKIMKey creates an RSA key and returns it as PKCS #8 and its public half as PKIX, both DER.
func NewDKIMKey() (private, public []byte, err error) {
	k, err := rsa.GenerateKey(rand.Reader, dkimKeyBits)
	if err != nil {
		return nil, nil, err
	}
	if private, err = x509.MarshalPKCS8PrivateKey(k); err != nil {
		return nil, nil, err
	}
	if public, err = x509.MarshalPKIXPublicKey(&k.PublicKey); err != nil {
		return nil, nil, err
	}
	return private, public, nil
}

func ParseDKIMKey(pkcs8 []byte) (crypto.Signer, error) {
	k, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, err
	}
	s, ok := k.(crypto.Signer)
	if !ok {
		return nil, errors.New("dkim: not a signing key")
	}
	return s, nil
}

// DKIMRecord is the TXT record that publishes a PKIX public key.
func DKIMRecord(public []byte) string {
	return "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public)
}

func DKIMRecordName(selector, domain string) string {
	return selector + "._domainkey." + strings.ToLower(domain)
}

// DKIMRecordMatches reports whether one of the TXT records at a selector's name publishes public.
func DKIMRecordMatches(txts []string, public []byte) bool {
	want := base64.StdEncoding.EncodeToString(public)
	for _, t := range txts {
		tags := dkimTags(t)
		if v, ok := tags["v"]; ok && v != "DKIM1" {
			continue
		}
		if strings.Join(strings.Fields(tags["p"]), "") == want {
			return true
		}
	}
	return false
}

// SignDKIM adds a relaxed/relaxed rsa-sha256 signature over DKIMHeaders and the body.
func SignDKIM(raw []byte, k DKIMKey) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(raw) + 512)
	err := dkim.Sign(&out, bytes.NewReader(raw), &dkim.SignOptions{
		Domain: k.Domain, Selector: k.Selector, Signer: k.Signer, Hash: crypto.SHA256,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed, BodyCanonicalization: dkim.CanonicalizationRelaxed,
		HeaderKeys: DKIMHeaders,
	})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func dkimTags(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ";") {
		k, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(val)
	}
	return out
}

type DKIMSignature struct {
	Domain   string
	Selector string
	Headers  []string
}

// ParseDKIMSignature reads d=, s= and h= of a DKIM-Signature value; h= is lower-cased.
func ParseDKIMSignature(v string) (DKIMSignature, bool) {
	tags := dkimTags(strings.Join(strings.Fields(v), " "))
	d := strings.ToLower(strings.TrimSuffix(tags["d"], "."))
	if d == "" {
		return DKIMSignature{}, false
	}
	sig := DKIMSignature{Domain: d, Selector: tags["s"]}
	for _, h := range strings.Split(tags["h"], ":") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			sig.Headers = append(sig.Headers, h)
		}
	}
	return sig, true
}

// CoversListUnsubscribe reports whether the signature covers both RFC 8058 headers.
func (s DKIMSignature) CoversListUnsubscribe() bool {
	return slices.Contains(s.Headers, "list-unsubscribe") && slices.Contains(s.Headers, "list-unsubscribe-post")
}

// OrgDomain is the organizational domain (RFC 7489 §3.2): the public suffix plus one label.
func OrgDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if d, err := publicsuffix.EffectiveTLDPlusOne(domain); err == nil {
		return d
	}
	return domain
}

// Aligned is DMARC relaxed alignment: both domains share their organizational domain.
func Aligned(a, b string) bool {
	return a != "" && b != "" && OrgDomain(a) == OrgDomain(b)
}

type AuthVerdict struct {
	SPF        string
	SPFDomain  string
	DKIM       []DKIMVerdict
	DMARC      string
	DMARCFound bool
}

type DKIMVerdict struct {
	Result string
	Domain string
}

var arComment = regexp.MustCompile(`\([^()]*\)`)

// TrustedAuth reads the topmost Authentication-Results only when its authserv-id is authservID,
// as TrustedDMARC does; ok is false otherwise.
func TrustedAuth(authResults, authservID string) (AuthVerdict, bool) {
	var v AuthVerdict
	if authservID == "" || !strings.EqualFold(AuthservID(authResults), authservID) {
		return v, false
	}
	clean := authResults
	for {
		next := arComment.ReplaceAllString(clean, " ")
		if next == clean {
			break
		}
		clean = next
	}
	_, results, err := authres.Parse(strings.Join(strings.Fields(clean), " "))
	if err != nil && len(results) == 0 {
		return v, true
	}
	mailFrom := false
	for _, r := range results {
		switch r := r.(type) {
		case *authres.SPFResult:
			// Receivers such as Cloudflare report the HELO check and the MAIL FROM check as two spf
			// results; DMARC uses the MAIL FROM one, so a HELO result only stands in when it is alone.
			if mailFrom || (r.From == "" && v.SPF != "") {
				continue
			}
			d := r.Helo
			if r.From != "" {
				mailFrom = true
				d = r.From
				if _, after, ok := strings.Cut(r.From, "@"); ok {
					d = after
				}
			}
			v.SPF = string(r.Value)
			v.SPFDomain = strings.ToLower(strings.TrimSuffix(d, "."))
		case *authres.DKIMResult:
			v.DKIM = append(v.DKIM, DKIMVerdict{Result: string(r.Value), Domain: strings.ToLower(strings.TrimSuffix(r.Domain, "."))})
		case *authres.DMARCResult:
			if !v.DMARCFound {
				v.DMARC, v.DMARCFound = string(r.Value), true
			}
		}
	}
	return v, true
}
