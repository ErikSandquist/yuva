package config

import "testing"

func TestAnalyticsMonthlyEventsDefault(t *testing.T) {
	for _, tc := range []struct {
		env    string
		signup bool
		want   int64
	}{
		{"", false, 1000000},
		{"", true, 1000000},
		{"250000", false, 250000},
		{"0", false, 0},
	} {
		t.Setenv("YUVA_ANALYTICS_MONTHLY_EVENTS", tc.env)
		c := Config{SignupOpen: tc.signup}
		if err := c.loadAnalytics(); err != nil {
			t.Fatal(err)
		}
		if c.Analytics.MonthlyEvents != tc.want {
			t.Errorf("env %q signup %v: %d, want %d", tc.env, tc.signup, c.Analytics.MonthlyEvents, tc.want)
		}
	}
}
