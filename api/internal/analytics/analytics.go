// Package analytics collects page views without cookies or stored addresses and keeps them in a
// Store. Handlers depend on the Store interface, never on its tables.
package analytics

import (
	"context"
	"strings"
	"time"
	"uuid"
)

type Kind int16

const (
	KindPageview   Kind = 1
	KindScreen     Kind = 2
	KindCustom     Kind = 3
	KindEngagement Kind = 4
)

type Channel int16

const (
	ChannelDirect       Channel = 1
	ChannelOrganic      Channel = 2
	ChannelPaidSearch   Channel = 3
	ChannelSocial       Channel = 4
	ChannelPaidSocial   Channel = 5
	ChannelEmail        Channel = 6
	ChannelReferral     Channel = 7
	ChannelAIAssistants Channel = 8
	ChannelInternal     Channel = 9
)

type Platform int16

const (
	PlatformWeb     Platform = 1
	PlatformIOS     Platform = 2
	PlatformAndroid Platform = 3
	PlatformServer  Platform = 4
)

// Drop reasons, as counted per property and day.
const (
	ReasonAccepted  = "accepted"
	ReasonBot       = "bot"
	ReasonRate      = "rate"
	ReasonOrigin    = "origin"
	ReasonInvalid   = "invalid"
	ReasonLate      = "late"
	ReasonOverLimit = "over_limit"
	ReasonBuffer    = "buffer"
	ReasonExcluded  = "excluded"
	ReasonScrubbed  = "scrubbed"
)

// VisitInfo describes a visit as its first event saw it; later events of the visit only move its
// end, exit page and counters.
type VisitInfo struct {
	Hostname     string
	Channel      Channel
	Source       string
	ReferrerHost string
	ReferrerPath string
	Acquisition  Acquisition
	Country      string
	Region       string
	City         string
	Device       int16
	Browser      string
	BrowserMajor int16
	OS           string
	OSMajor      int16
	Screen       int16
	Language     string
	Platform     Platform
	AppVersion   string
}

type Event struct {
	WorkspaceID uuid.UUID
	PropertyID  uuid.UUID
	Day         time.Time
	TS          time.Time
	Visit       int64
	Visitor     int64
	Kind        Kind
	Name        string
	Hostname    string
	Path        string
	EngagedMS   *int32
	Scroll      *int16
	Interactive bool
	Info        *VisitInfo
}

// Count is a number of events of a property and day for one reason.
type Count struct {
	WorkspaceID uuid.UUID
	PropertyID  uuid.UUID
	Day         time.Time
	Reason      string
	N           int64
}

type Batch struct {
	Events []Event
	Drops  []Count
}

// IngestResult is each workspace's analytics events of the month after the batch.
type IngestResult struct {
	Month time.Time
	Usage map[uuid.UUID]int64
	// Late counts events written to days that were already rolled up.
	Late int64
}

type Store interface {
	// EnsurePartitions makes the raw tables' partitions of the days from and to exist.
	EnsurePartitions(ctx context.Context, from, to time.Time) error
	// Ingest writes a batch in one transaction: events, visits, usage, ingest counts and the days
	// to roll up.
	Ingest(ctx context.Context, b Batch, now time.Time) (IngestResult, error)
	// Salt returns the salt of a property's day, storing fresh when the day has none yet.
	Salt(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, fresh []byte, expires time.Time) ([]byte, time.Time, error)
	// LatestVisit is the visitor's last visit of the day, if any.
	LatestVisit(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, visitor int64) (visit int64, ended time.Time, ok bool, err error)
	VisitorsToday(ctx context.Context, workspaceID uuid.UUID, days map[uuid.UUID]time.Time) (map[uuid.UUID]int64, error)
	// DeleteRaw deletes up to limit rows of each raw table of a property's day and says how many went.
	DeleteRaw(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, limit int) (int64, error)
}

// LocalDay is the date of t in loc, as midnight UTC, the form days are stored and compared in.
func LocalDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DayEnd is when a local day ends.
func DayEnd(day time.Time, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, loc)
}

var emailMediums = []string{"email", "e-mail", "newsletter"}

// ClassifyChannel picks a visit's channel from its referrer and acquisition. internal reports a
// referrer on another of the property's domains.
func ClassifyChannel(referrerHost string, a Acquisition, internal bool) Channel {
	medium := strings.ToLower(a.Medium)
	switch {
	case medium == "cpc" || medium == "ppc" || strings.HasPrefix(medium, "paid"):
		return ChannelPaidSearch
	case containsFold(emailMediums, medium):
		return ChannelEmail
	case internal:
		return ChannelInternal
	case referrerHost != "" || a.Source != "":
		return ChannelReferral
	}
	return ChannelDirect
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
