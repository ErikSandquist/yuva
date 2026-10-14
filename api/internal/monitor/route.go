package monitor

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	// OtherValue is what a value past its cap is counted as; Unmatched the route of a 404 or 405
	// that named no pattern; Scrubbed an e-mail-shaped consumer.
	OtherValue     = "(other)"
	Unmatched      = "(unmatched)"
	Scrubbed       = "(scrubbed)"
	IDSegment      = "{id}"
	MaxRoute       = 300
	maxSegment     = 40
	minTokenLength = 16
)

var methods = []string{"_OTHER", "GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}

// Method is a request method's code: the RFC 9110 methods and PATCH, else 0 (_OTHER).
func Method(m string) int16 {
	m = strings.ToUpper(strings.TrimSpace(m))
	for i, name := range methods[1:] {
		if m == name {
			return int16(i + 1)
		}
	}
	return 0
}

func MethodName(code int16) string {
	if code < 0 || int(code) >= len(methods) {
		return methods[0]
	}
	return methods[code]
}

// StatusClass is a status's hundreds, 0 for no response.
func StatusClass(status int16) int16 { return status / 100 }

func StatusClassName(class int16) string {
	if class <= 0 {
		return "none"
	}
	return strconv.Itoa(int(class)) + "xx"
}

// IsError: a 5xx, or no response at all. IsClientError: a 4xx.
func IsError(status int16) bool       { return status == 0 || (status >= 500 && status <= 599) }
func IsClientError(status int16) bool { return status >= 400 && status <= 499 }

var (
	uuidSegment  = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)
	dateSegment  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$`)
	hexSegment   = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	tokenSegment = regexp.MustCompile(`^[A-Za-z0-9_-]+={0,2}$`)
	emailSegment = regexp.MustCompile(`^[^@\s]+(@|%40)[^@\s]+\.[^@\s.]+$`)
)

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// idLike: a segment that names one record rather than a part of the API. A token of 16 or more
// letters, digits, - and _ counts only with two digits or more, or with both cases and a digit,
// so words such as user-preferences stay part of the route.
func idLike(seg string) bool {
	switch {
	case seg == "":
		return false
	case len(seg) > maxSegment, isNumber(seg), uuidSegment.MatchString(seg), dateSegment.MatchString(seg), emailSegment.MatchString(seg):
		return true
	case len(seg) < minTokenLength:
		return false
	case hexSegment.MatchString(seg):
		return true
	case !tokenSegment.MatchString(seg):
		return false
	}
	digits, upper, lower := 0, false, false
	for _, r := range seg {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		}
	}
	return digits >= 2 || (digits == 1 && upper && lower)
}

// patternSegment: a router's placeholder, kept as it is.
func patternSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") || strings.HasPrefix(seg, "[") ||
		strings.HasPrefix(seg, "<")
}

// InferRoute turns a path into a pattern: the query and fragment are cut and every segment that
// looks like an id becomes {id}. It also cleans a router's pattern, so an id put into a route by
// mistake never becomes a route of its own.
func InferRoute(path string) string {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	segs := strings.Split(path[1:], "/")
	for i, seg := range segs {
		if !patternSegment(seg) && idLike(seg) {
			segs[i] = IDSegment
		}
	}
	return "/" + strings.Join(segs, "/")
}

var emailShape = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s.]+$`)

// ScrubConsumer keeps a consumer's name unless it is shaped like an e-mail address: a consumer is
// an account or an API client, never a person's address.
func ScrubConsumer(v string) string {
	if strings.Contains(v, "@") && emailShape.MatchString(v) {
		return Scrubbed
	}
	return v
}
