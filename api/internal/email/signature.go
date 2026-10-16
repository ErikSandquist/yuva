package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"strconv"
	"strings"
	"time"
)

const ReplayWindow = 5 * time.Minute

var (
	ErrBadSignature   = errors.New("signature does not match")
	ErrStaleTimestamp = errors.New("timestamp is outside the replay window")
)

// Sign signs an /ingress/email request with the current version, v3: one MAC over the
// timestamp, both envelope addresses and the body length, which the server checks before it reads
// the body, and one that adds the body.
func Sign(secret, timestamp, envelopeTo, envelopeFrom string, body []byte) string {
	head := v3Head(timestamp, envelopeTo, envelopeFrom, int64(len(body)))
	headMAC := ingressMAC(secret, head)
	bodyMAC := ingressMAC(secret, head+"\n")
	bodyMAC.Write(body)
	return "v3=" + hex.EncodeToString(headMAC.Sum(nil)) + "." + hex.EncodeToString(bodyMAC.Sum(nil))
}

// v3Head separates its fields with newlines, which no header value can contain, so no two
// requests share one.
func v3Head(timestamp, envelopeTo, envelopeFrom string, length int64) string {
	return "v3\n" + timestamp + "\n" + envelopeTo + "\n" + envelopeFrom + "\n" + strconv.FormatInt(length, 10)
}

// SignV2 is the deprecated version whose only MAC covers the body, so the server reads the body
// before it knows the request is genuine.
func SignV2(secret, timestamp, envelopeTo, envelopeFrom string, body []byte) string {
	mac := ingressMAC(secret, timestamp+"."+envelopeTo+"."+envelopeFrom+".")
	mac.Write(body)
	return "v2=" + hex.EncodeToString(mac.Sum(nil))
}

// SignV1 is the deprecated version that leaves the envelope sender unsigned.
func SignV1(secret, timestamp, envelopeTo string, body []byte) string {
	mac := ingressMAC(secret, timestamp+"."+envelopeTo+".")
	mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func ingressMAC(secret, prefix string) hash.Hash {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(prefix))
	return mac
}

// IngressCheck verifies an /ingress/email request (edge/README.md) while its body is read: the
// headers, the timestamp and for v3 the header MAC are checked before the first byte, the body MAC
// once the body is complete.
type IngressCheck struct {
	mac  hash.Hash
	want []byte
	v1   bool
}

// Legacy names the deprecated signature versions still accepted.
type Legacy struct{ V1, V2 bool }

// StartIngress checks the signature headers. contentLength is the declared body length, -1 when
// unknown; v3 requires it. v1 and v2 are refused unless legacy allows them.
func StartIngress(secret, timestamp, envelopeTo, envelopeFrom, signature string, contentLength int64, now time.Time, legacy Legacy) (*IngressCheck, error) {
	version, hexSig, ok := strings.Cut(strings.TrimSpace(signature), "=")
	if !ok {
		return nil, ErrBadSignature
	}
	var headSig string
	switch version {
	case "v3":
		if headSig, hexSig, ok = strings.Cut(hexSig, "."); !ok || contentLength < 0 {
			return nil, ErrBadSignature
		}
	case "v2":
		ok = legacy.V2
	case "v1":
		ok = legacy.V1
	default:
		ok = false
	}
	if !ok {
		return nil, ErrBadSignature
	}
	want, err := hex.DecodeString(hexSig)
	if err != nil || len(want) != sha256.Size {
		return nil, ErrBadSignature
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || ts < 0 {
		return nil, ErrBadSignature
	}
	head := v3Head(timestamp, envelopeTo, envelopeFrom, contentLength)
	if version == "v3" {
		got, err := hex.DecodeString(headSig)
		if err != nil || !hmac.Equal(got, ingressMAC(secret, head).Sum(nil)) {
			return nil, ErrBadSignature
		}
	}
	if d := now.Sub(time.Unix(ts, 0)); d > ReplayWindow || d < -ReplayWindow {
		return nil, ErrStaleTimestamp
	}
	c := &IngressCheck{want: want, v1: version == "v1"}
	switch version {
	case "v3":
		c.mac = ingressMAC(secret, head+"\n")
	case "v2":
		c.mac = ingressMAC(secret, timestamp+"."+envelopeTo+"."+envelopeFrom+".")
	default:
		c.mac = ingressMAC(secret, timestamp+"."+envelopeTo+".")
	}
	return c, nil
}

func (c *IngressCheck) Write(p []byte) (int, error) { return c.mac.Write(p) }

// Verify compares the MAC over everything written with the signature, in constant time.
func (c *IngressCheck) Verify() error {
	if !hmac.Equal(c.want, c.mac.Sum(nil)) {
		return ErrBadSignature
	}
	return nil
}

// V1 reports a deprecated v1 signature, under which the envelope sender is not authenticated.
func (c *IngressCheck) V1() bool { return c.v1 }

// VerifyIngress checks a request whose body is already in memory.
func VerifyIngress(secret, timestamp, envelopeTo, envelopeFrom, signature string, body []byte, now time.Time, legacy Legacy) (*IngressCheck, error) {
	c, err := StartIngress(secret, timestamp, envelopeTo, envelopeFrom, signature, int64(len(body)), now, legacy)
	if err != nil {
		return nil, err
	}
	c.mac.Write(body)
	return c, c.Verify()
}
