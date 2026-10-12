package analytics

import (
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"sync"

	useragent "github.com/medama-io/go-useragent"
	"github.com/medama-io/go-useragent/agents"

	"github.com/productdevbook/yuva/api/internal/analytics/lists"
)

const (
	DeviceUnknown int16 = 0
	DeviceDesktop int16 = 1
	DeviceMobile  int16 = 2
	DeviceTablet  int16 = 3

	minUserAgent   = 10
	maxAgentCache  = 20_000
	maxAgentLength = 1000
)

// Agent is what a request's user agent and client hints say about the visitor's software.
type Agent struct {
	Browser      string
	BrowserMajor int16
	OS           string
	OSMajor      int16
	Device       int16
	Bot          bool
}

var (
	uaParser    = sync.OnceValue(useragent.NewParser)
	crawlers    = sync.OnceValue(func() *botMatcher { return newBotMatcher(lists.Crawlers) })
	agentCache  = map[string]Agent{}
	agentMu     sync.Mutex
	androidVer  = regexp.MustCompile(`Android (\d+)`)
	iosVer      = regexp.MustCompile(`(?:iPhone|CPU|iPad) OS (\d+)_`)
	macVer      = regexp.MustCompile(`Mac OS X (\d+)[_.]`)
	windowsVer  = regexp.MustCompile(`Windows NT (\d+\.\d+)`)
	windowsName = map[string]int16{"10.0": 10, "6.3": 8, "6.2": 8, "6.1": 7}
	hintOS      = map[string]string{"windows": "Windows", "macos": "MacOS", "android": "Android", "ios": "iOS", "chrome os": "ChromeOS", "chromeos": "ChromeOS", "linux": "Linux"}
)

// ParseAgent reads the user agent and the low-entropy client hints a browser sends anyway
// (Sec-CH-UA-Platform, Sec-CH-UA-Mobile). Empty and very short agents, crawlers and agents the
// parser knows as bots are bots.
func ParseAgent(ua, platformHint, mobileHint string) Agent {
	if len(ua) > maxAgentLength {
		ua = ua[:maxAgentLength]
	}
	key := ua + "\x00" + platformHint + "\x00" + mobileHint
	agentMu.Lock()
	a, ok := agentCache[key]
	agentMu.Unlock()
	if ok {
		return a
	}
	a = parseAgent(ua, platformHint, mobileHint)
	agentMu.Lock()
	if len(agentCache) >= maxAgentCache {
		agentCache = map[string]Agent{}
	}
	agentCache[key] = a
	agentMu.Unlock()
	return a
}

func parseAgent(ua, platformHint, mobileHint string) Agent {
	if len(strings.TrimSpace(ua)) < minUserAgent || crawlers().match(ua) {
		return Agent{Bot: true}
	}
	p := uaParser().Parse(ua)
	if p.IsBot() {
		return Agent{Bot: true}
	}
	a := Agent{Browser: string(p.Browser()), OS: string(p.OS())}
	if v, err := strconv.Atoi(p.BrowserVersionMajor()); err == nil && v > 0 && v < 1<<15 {
		a.BrowserMajor = int16(v)
	}
	switch p.Device() {
	case agents.DeviceDesktop:
		a.Device = DeviceDesktop
	case agents.DeviceMobile:
		a.Device = DeviceMobile
	case agents.DeviceTablet:
		a.Device = DeviceTablet
	}
	if os, ok := hintOS[strings.ToLower(strings.Trim(platformHint, `" `))]; ok {
		a.OS = os
	}
	if mobileHint == "?1" && a.Device != DeviceTablet {
		a.Device = DeviceMobile
	}
	a.OSMajor = osMajor(a.OS, ua)
	return a
}

func osMajor(os, ua string) int16 {
	var m []string
	switch os {
	case string(agents.OSAndroid):
		m = androidVer.FindStringSubmatch(ua)
	case string(agents.OSIOS):
		m = iosVer.FindStringSubmatch(ua)
	case string(agents.OSMacOS):
		m = macVer.FindStringSubmatch(ua)
	case string(agents.OSWindows):
		if w := windowsVer.FindStringSubmatch(ua); w != nil {
			return windowsName[w[1]]
		}
	}
	if m == nil {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil || v >= 1<<15 {
		return 0
	}
	return int16(v)
}

// botMatcher matches the crawler list without running one large regular expression per request
// (which costs milliseconds): plain patterns are found by their first two bytes, and the few real
// expressions run only when a literal they require is present.
type botMatcher struct {
	literals map[uint16][]string
	exprs    []guardedExpr
}

type guardedExpr struct {
	need string
	re   *regexp.Regexp
}

func newBotMatcher(list string) *botMatcher {
	m := &botMatcher{literals: map[uint16][]string{}}
	for _, line := range strings.Split(list, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := syntax.Parse(line, syntax.Perl)
		if err != nil {
			continue
		}
		parsed = parsed.Simplify()
		alts := []*syntax.Regexp{parsed}
		if parsed.Op == syntax.OpAlternate {
			alts = parsed.Sub
		}
		for _, alt := range alts {
			if alt.Op == syntax.OpLiteral && alt.Flags&syntax.FoldCase == 0 && len(string(alt.Rune)) >= 2 {
				m.addLiteral(string(alt.Rune))
				continue
			}
			re, err := regexp.Compile(alt.String())
			if err != nil {
				continue
			}
			m.exprs = append(m.exprs, guardedExpr{need: longestLiteral(alt), re: re})
		}
	}
	return m
}

func (m *botMatcher) addLiteral(s string) {
	k := uint16(s[0])<<8 | uint16(s[1])
	m.literals[k] = append(m.literals[k], s)
}

func longestLiteral(re *syntax.Regexp) string {
	best := ""
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase == 0 {
			best = string(re.Rune)
		}
	case syntax.OpConcat, syntax.OpCapture:
		for _, sub := range re.Sub {
			if l := longestLiteral(sub); len(l) > len(best) {
				best = l
			}
		}
	}
	return best
}

func (m *botMatcher) match(ua string) bool {
	for i := 0; i+1 < len(ua); i++ {
		for _, l := range m.literals[uint16(ua[i])<<8|uint16(ua[i+1])] {
			if strings.HasPrefix(ua[i:], l) {
				return true
			}
		}
	}
	for _, e := range m.exprs {
		if (e.need == "" || strings.Contains(ua, e.need)) && e.re.MatchString(ua) {
			return true
		}
	}
	return false
}
