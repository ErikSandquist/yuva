package migrate

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

func TestThrottleDailyBudgetWithAFakeClock(t *testing.T) {
	gmail, _ := LookupPreset(PresetGmail)
	b := gmail.Budget
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var th Throttle
	if until := th.Wait(b, now, 10<<20); !until.IsZero() {
		t.Fatalf("an empty window waits until %s", until)
	}
	for th.WindowBytes+(100<<20) <= b.BytesPerDay {
		th.Read(now, 100<<20)
		now = now.Add(time.Minute)
	}
	until := th.Wait(b, now, 100<<20)
	if !until.Equal(th.WindowStart.Add(24 * time.Hour)) {
		t.Fatalf("a full window waits until %s, want %s", until, th.WindowStart.Add(24*time.Hour))
	}
	saved, err := json.Marshal(th)
	if err != nil {
		t.Fatal(err)
	}
	var restarted Throttle
	if err := json.Unmarshal(saved, &restarted); err != nil {
		t.Fatal(err)
	}
	if got := restarted.Wait(b, now.Add(time.Hour), 100<<20); !got.Equal(until) {
		t.Fatalf("after a restart the budget waits until %s, want %s", got, until)
	}
	if got := restarted.Wait(b, until.Add(time.Second), 100<<20); !got.IsZero() || restarted.WindowBytes != 0 {
		t.Fatalf("the next window still waits: %s, %d bytes", got, restarted.WindowBytes)
	}
	if !restarted.WindowEnd(until.Add(time.Second)).Equal(until.Add(time.Second).Add(24 * time.Hour)) {
		t.Fatal("the window did not roll over")
	}
	other, _ := LookupPreset(PresetIMAP)
	if got := (&Throttle{}).Wait(other.Budget, now, 1<<40); !got.IsZero() {
		t.Fatalf("a server without a daily budget waits until %s", got)
	}
	if d := Pace(Budget{BytesPerSecond: 4 << 20}, 8<<20, time.Second); d != time.Second {
		t.Fatalf("pace %s", d)
	}
}

func TestThrottleBackoff(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var th Throttle
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 15 * time.Minute, 15 * time.Minute}
	for i, w := range want {
		if got := th.Retry(now).Sub(now); got != w {
			t.Fatalf("retry %d waits %s, want %s", i+1, got, w)
		}
	}
}

// Every answer of the corpus tells the engine what to do, and nothing secret survives into the
// stored error.
func TestErrorCorpusTrouble(t *testing.T) {
	files, _ := filepath.Glob("testdata/errors/*.txt")
	const password = "Hunter2-App-Password"
	const blob = "dXNlcj1zb21ldXNlckBleGFtcGxlLmNvbQFhdXRoPUJlYXJlcg"
	for _, f := range files {
		fields := map[string]string{}
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			k, v, _ := strings.Cut(sc.Text(), ": ")
			fields[k] = v
		}
		fh.Close()
		typ, rest, _ := strings.Cut(fields["answer"], " ")
		se := &imapread.ServerError{Type: typ, Text: rest + " " + password + " " + blob}
		if strings.HasPrefix(rest, "[") {
			code, text, _ := strings.Cut(rest[1:], "] ")
			se.Code, se.Text = code, text+" "+password+" "+blob
		}
		var answer error = se
		if fields["mechanism"] == "xoauth2" {
			answer = &imapread.Error{Step: imapread.StepSignIn, Err: &imapread.TokenRefusedError{ServerError: se}}
		}
		kind, code, detail := Classify(answer, fields["host"], password)
		if fields["code"] == CodeXOAUTH2Failed && code != CodeXOAUTH2Failed {
			t.Errorf("%s: code %s, want %s", f, code, CodeXOAUTH2Failed)
		}
		var want []Trouble
		switch fields["code"] {
		case CodeAuthFailed, CodeAppPasswordRequired, CodeIMAPDisabled, CodeBasicAuthDisabled, CodeWebLoginRequired, CodeXOAUTH2Failed:
			want = []Trouble{TroubleReconnect}
		case CodeThrottled, CodeTooManyConnections:
			want = []Trouble{TroubleLimit, TroubleDaily}
		default:
			want = []Trouble{TroubleNetwork, TroubleLimit}
		}
		if !containsTrouble(want, kind) {
			t.Errorf("%s: %s, want one of %v", f, kind, want)
		}
		if strings.Contains(detail, password) || strings.Contains(detail, blob) {
			t.Errorf("%s: a secret survived in %q", f, detail)
		}
	}
}

func containsTrouble(list []Trouble, v Trouble) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
