package migrate

import (
	"errors"
	"regexp"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate/helpdesk"
	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

const (
	window     = 24 * time.Hour
	backoffMin = 30 * time.Second
	backoffMax = 15 * time.Minute
)

// Throttle is a migration's use of its budget, kept on the migration row so a restart does not
// reset it.
type Throttle struct {
	WindowStart time.Time `json:"window_start,omitzero"`
	WindowBytes int64     `json:"window_bytes,omitempty"`
	Backoff     int       `json:"backoff,omitempty"`
	// Requests is a help desk's request budget.
	Requests *helpdesk.Budget `json:"requests,omitempty"`
}

func (t *Throttle) roll(now time.Time) {
	if t.WindowStart.IsZero() || !now.Before(t.WindowStart.Add(window)) {
		t.WindowStart, t.WindowBytes = now, 0
	}
}

// Wait is when next more bytes may be read within a daily budget; zero when now.
func (t *Throttle) Wait(b Budget, now time.Time, next int64) time.Time {
	if b.BytesPerDay <= 0 {
		return time.Time{}
	}
	t.roll(now)
	if t.WindowBytes > 0 && t.WindowBytes+next > b.BytesPerDay {
		return t.WindowStart.Add(window)
	}
	return time.Time{}
}

func (t *Throttle) Read(now time.Time, n int64) {
	t.roll(now)
	t.WindowBytes += n
}

// WindowEnd is when the daily window rolls over.
func (t *Throttle) WindowEnd(now time.Time) time.Time {
	t.roll(now)
	return t.WindowStart.Add(window)
}

// Retry is the next wait after a limit answer or a failed connection: 30 seconds, doubling, at
// most 15 minutes.
func (t *Throttle) Retry(now time.Time) time.Time {
	d := backoffMin << min(t.Backoff, 5)
	t.Backoff++
	return now.Add(min(d, backoffMax))
}

// Pace is how long to wait after reading n bytes in took, to stay within bytes per second.
func Pace(b Budget, n int64, took time.Duration) time.Duration {
	if b.BytesPerSecond <= 0 || n <= 0 {
		return 0
	}
	want := time.Duration(float64(n) / float64(b.BytesPerSecond) * float64(time.Second))
	return max(want-took, 0)
}

type Trouble string

const (
	TroubleReconnect Trouble = "reconnect"
	TroubleLimit     Trouble = "limit"
	TroubleDaily     Trouble = "daily"
	TroubleNetwork   Trouble = "network"
	TroubleRefused   Trouble = "refused"
)

var dailyRe = regexp.MustCompile(`(?i)bandwidth|download limit|daily limit|quota`)

// Classify tells what a failure while reading the source asks for: reconnect (credentials no
// longer work), wait for a limit or for the daily window, retry after a network problem, or stop
// because the wire guard refused a command. code and detail are for the migration's errors,
// scrubbed of secrets.
func Classify(err error, host string, secrets ...string) (Trouble, string, string) {
	var refused *imapread.RefusedError
	if errors.As(err, &refused) {
		return TroubleRefused, "source_write_refused", "refused to send " + refused.Command
	}
	if trouble, code, detail, ok := classifyHelpdesk(err); ok {
		return trouble, code, Scrub(detail, secrets...)
	}
	var se *imapread.ServerError
	if errors.As(err, &se) {
		code, detail := ClassifyAnswer(se, host)
		detail = Scrub(detail, secrets...)
		switch code {
		case CodeAuthFailed, CodeAppPasswordRequired, CodeIMAPDisabled, CodeBasicAuthDisabled, CodeWebLoginRequired:
			return TroubleReconnect, "auth_failed", detail
		case CodeThrottled, CodeTooManyConnections:
			if dailyRe.MatchString(se.Text) {
				return TroubleDaily, "throttled", detail
			}
			return TroubleLimit, "throttled", detail
		}
		if se.Type == "BYE" {
			return TroubleLimit, "throttled", detail
		}
		return TroubleNetwork, "source_unreachable", detail
	}
	var ie *imapread.Error
	if (errors.As(err, &ie) && ie.Step == imapread.StepTLS) || errors.Is(err, ErrNotAllowed) {
		return TroubleReconnect, "source_unreachable", Scrub(err.Error(), secrets...)
	}
	return TroubleNetwork, "source_unreachable", Scrub(err.Error(), secrets...)
}

// classifyHelpdesk tells what a help desk's failure asks for.
func classifyHelpdesk(err error) (Trouble, string, string, bool) {
	var refused *helpdesk.RefusedError
	var auth *helpdesk.AuthError
	var token *helpdesk.TokenError
	var limit *helpdesk.LimitError
	var status *helpdesk.StatusError
	switch {
	case errors.As(err, &refused):
		return TroubleRefused, "source_write_refused", "refused to send " + refused.Method, true
	case errors.As(err, &auth), errors.As(err, &token):
		return TroubleReconnect, "auth_failed", err.Error(), true
	case errors.As(err, &limit):
		return TroubleLimit, "throttled", err.Error(), true
	case errors.As(err, &status) && status.Status == 403:
		return TroubleReconnect, "auth_failed", err.Error(), true
	case errors.As(err, &status):
		return TroubleNetwork, "source_unreachable", err.Error(), true
	}
	return "", "", "", false
}
