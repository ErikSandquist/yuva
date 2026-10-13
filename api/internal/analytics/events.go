package analytics

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"net/url"
	"regexp"
	"strings"
)

const (
	MaxEventName  = 120
	MaxProps      = 30
	MaxPropKey    = 300
	MaxPropValue  = 2000
	maxMinorUnits = 1_000_000_000_000_000

	EventOutbound = "outbound_click"
	EventFile     = "file_download"
	EventNotFound = "not_found"
	EventChatOpen = "chat_open"
	PropURL       = "url"
)

// BuiltinEvents are the event names the script sends itself; they are custom events with rules.
var BuiltinEvents = []string{EventOutbound, EventFile, EventNotFound, EventChatOpen}

// ReservedKey reports a prop key or event name that only Yuva may use.
func ReservedKey(s string) bool {
	return strings.HasPrefix(s, "$") || strings.HasPrefix(s, "yuva_")
}

// EventName checks a custom or built-in event's name: 1 to 120 characters after cleaning, and
// neither reserved nor a name with a meaning of its own.
func EventName(raw string) (string, error) {
	n := Clean(raw, 4*MaxEventName)
	switch {
	case n == "" || len([]rune(n)) > MaxEventName:
		return "", errors.New("n must be 1 to 120 characters")
	case n == "pageview" || n == "engagement" || n == "screen":
		return "", errors.New("n " + n + " is not a custom event")
	case ReservedKey(n):
		return "", errors.New("event names starting with $ or yuva_ are reserved")
	}
	return n, nil
}

// Props are an event's props as stored; Scrubbed counts the values dropped for looking like an
// e-mail address.
type Props struct {
	Values   map[string]string
	Scrubbed int
}

// ParseProps reads an event's `p`: at most 30 keys of 1 to 300 characters, none reserved, with
// string, number or boolean values kept as text of at most 2,000 characters.
func ParseProps(raw json.RawMessage) (Props, error) {
	var out Props
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return out, errors.New("p must be an object of strings, numbers and booleans")
	}
	if len(m) > MaxProps {
		return out, errors.New("p may hold at most 30 props")
	}
	for k, v := range m {
		key := Clean(k, 4*MaxPropKey)
		if key == "" || len([]rune(key)) > MaxPropKey {
			return out, errors.New("prop keys must be 1 to 300 characters")
		}
		if ReservedKey(key) {
			return out, errors.New("prop keys starting with $ or yuva_ are reserved")
		}
		var s string
		switch x := v.(type) {
		case string:
			s = x
		case json.Number:
			s = x.String()
		case bool:
			s = "false"
			if x {
				s = "true"
			}
		default:
			return out, errors.New("prop values must be strings, numbers or booleans")
		}
		s = Clean(s, MaxPropValue)
		if looksLikeEmail(s) {
			out.Scrubbed++
			continue
		}
		if out.Values == nil {
			out.Values = map[string]string{}
		}
		out.Values[key] = s
	}
	return out, nil
}

// LinkURL is the `url` of an outbound or download event without its query and fragment, with
// e-mail-shaped path segments replaced.
func LinkURL(raw string) (*url.URL, bool) {
	if len(raw) > MaxURLLength {
		return nil, false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, false
	}
	path, _ := scrubPath(u.Path)
	return &url.URL{Scheme: u.Scheme, Host: strings.ToLower(u.Host), Path: path}, true
}

// currencyDigits are the ISO 4217 currencies whose minor unit is not a hundredth.
var currencyDigits = map[string]int{
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "ISK": 0, "JPY": 0, "KMF": 0, "KRW": 0, "PYG": 0, "RWF": 0, "UGX": 0, "UYI": 0,
	"VND": 0, "VUV": 0, "XAF": 0, "XOF": 0, "XPF": 0,
	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "LYD": 3, "OMR": 3, "TND": 3,
	"CLF": 4, "UYW": 4,
}

// CurrencyDigits is how many decimal digits a currency's minor unit has.
func CurrencyDigits(code string) int {
	if d, ok := currencyDigits[code]; ok {
		return d
	}
	return 2
}

var (
	currencyCode  = regexp.MustCompile(`^[A-Z]{3}$`)
	decimalAmount = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
)

// Revenue is an amount in a currency's minor units.
type Revenue struct {
	Minor    int64
	Currency string
}

// ParseRevenue reads an event's `$`: an amount (a decimal string or number with at most the
// currency's minor-unit digits) and an ISO 4217 code.
func ParseRevenue(raw json.RawMessage) (*Revenue, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var v struct {
		A json.RawMessage `json:"a"`
		C string          `json:"c"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, errors.New("$ must be {a: amount, c: currency}")
	}
	code := strings.ToUpper(strings.TrimSpace(v.C))
	if !currencyCode.MatchString(code) {
		return nil, errors.New("$.c must be an ISO 4217 currency code such as EUR")
	}
	amount := strings.TrimSpace(string(v.A))
	if s, err := strconvUnquote(amount); err == nil {
		amount = strings.TrimSpace(s)
	}
	if len(amount) > 32 || !decimalAmount.MatchString(amount) {
		return nil, errors.New("$.a must be a decimal amount such as 29.90")
	}
	digits := CurrencyDigits(code)
	if _, frac, ok := strings.Cut(amount, "."); ok && len(strings.TrimRight(frac, "0")) > digits {
		return nil, errors.New("$.a has more decimals than " + code + " allows")
	}
	r, ok := new(big.Rat).SetString(amount)
	if !ok {
		return nil, errors.New("$.a must be a decimal amount such as 29.90")
	}
	r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)))
	if !r.IsInt() || r.Num().CmpAbs(big.NewInt(maxMinorUnits)) > 0 {
		return nil, errors.New("$.a is out of range")
	}
	return &Revenue{Minor: r.Num().Int64(), Currency: code}, nil
}

func strconvUnquote(s string) (string, error) {
	var out string
	err := json.Unmarshal([]byte(s), &out)
	return out, err
}
