// Package monitor counts an API's requests: records are added up in memory per minute and hour and
// written as rows of counters and duration sketches, never a row per request. Handlers depend on
// the Store interface, never on its tables.
package monitor

import (
	"context"
	"time"
	"uuid"
)

type Source int16

const (
	SourceServer Source = 1
	SourceOTLP   Source = 2
	SourceApp    Source = 3
)

type Platform int16

const (
	PlatformNone    Platform = 0
	PlatformIOS     Platform = 1
	PlatformAndroid Platform = 2
)

// Extra dimensions, each kept by local hour with route, method and status class.
const (
	DimConsumer   int16 = 1
	DimCountry    int16 = 2
	DimAppVersion int16 = 3
)

// Name kinds of the dictionaries and the caps on them.
const (
	KindRoute       = "route"
	KindEnvironment = "environment"
	KindService     = "service"
	KindAPIVersion  = "api_version"
	KindConsumer    = "consumer"
	KindAppVersion  = "app_version"
	KindCountry     = "country"
)

var Caps = map[string]int{
	KindRoute: 1000, KindConsumer: 10_000, KindAPIVersion: 50, KindAppVersion: 200, KindEnvironment: 20, KindService: 100,
}

// DimRowsPerHour is how many extra-dimension rows a property's hour holds; later new combinations
// count as (other).
const DimRowsPerHour = 50_000

var dimKinds = map[int16]string{DimConsumer: KindConsumer, DimCountry: KindCountry, DimAppVersion: KindAppVersion}

// DefaultEnvironment is a record's environment when it names none.
const DefaultEnvironment = "production"

// Drop reasons, as counted per property and day.
const (
	ReasonAccepted  = "accepted"
	ReasonInvalid   = "invalid"
	ReasonLate      = "late"
	ReasonRate      = "rate"
	ReasonOverLimit = "over_limit"
	ReasonBuffer    = "buffer"
)

// Record is one request once checked. Hour is the local hour the request started in and Day that
// hour's local day; Weight is how many requests it stands for.
type Record struct {
	Time        time.Time
	Hour        time.Time
	Day         time.Time
	Environment string
	Service     string
	Route       string
	Inferred    bool
	Method      int16
	Status      int16
	APIVersion  string
	Source      Source
	Platform    Platform
	DurationUS  int64
	// RequestBytes and ResponseBytes are -1 when unknown.
	RequestBytes  int64
	ResponseBytes int64
	Consumer      string
	Country       string
	AppVersion    string
	Weight        int64
}

// LocalHour is the first instant of the local hour t is in: the hour starts when loc's clock last
// showed a full hour, so both of the hours a clock repeats in autumn are hours of their own.
func LocalHour(t time.Time, loc *time.Location) time.Time {
	lt := t.In(loc)
	return t.Add(-time.Duration(lt.Minute())*time.Minute - time.Duration(lt.Second())*time.Second - time.Duration(lt.Nanosecond())).UTC()
}

// LocalDay is the date of t in loc, as midnight UTC.
func LocalDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DayStart is when a local day begins.
func DayStart(day time.Time, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).UTC()
}

// Property is what aggregation needs of an API property.
type Property struct {
	WorkspaceID uuid.UUID
	ID          uuid.UUID
	Location    *time.Location
}

// Agg is what rows add up: exact counters and the duration and response size sketches.
type Agg struct {
	Requests      int64
	Errors        int64
	ClientErrors  int64
	Sampled       int64
	DurationSum   int64
	DurationMin   int64
	DurationMax   int64
	Durations     Sketch
	RequestBytes  int64
	ResponseBytes int64
	Sizes         Sketch
}

func NewAgg() Agg { return Agg{Durations: NewSketch(DurationScale), Sizes: NewSketch(SizeScale)} }

func (a *Agg) Add(r *Record) {
	w := max(r.Weight, 1)
	if a.Requests == 0 || r.DurationUS < a.DurationMin {
		a.DurationMin = r.DurationUS
	}
	a.DurationMax = max(a.DurationMax, r.DurationUS)
	a.Requests += w
	if w > 1 {
		a.Sampled += w
	}
	if IsError(r.Status) {
		a.Errors += w
	} else if IsClientError(r.Status) {
		a.ClientErrors += w
	}
	a.DurationSum += r.DurationUS * w
	a.Durations.Add(r.DurationUS, uint64(w))
	if r.RequestBytes > 0 {
		a.RequestBytes += r.RequestBytes * w
	}
	if r.ResponseBytes >= 0 {
		a.ResponseBytes += r.ResponseBytes * w
		a.Sizes.Add(r.ResponseBytes, uint64(w))
	}
}

func (a *Agg) Merge(b *Agg) {
	if b.Requests == 0 {
		return
	}
	if a.Requests == 0 || b.DurationMin < a.DurationMin {
		a.DurationMin = b.DurationMin
	}
	a.DurationMax = max(a.DurationMax, b.DurationMax)
	a.Requests += b.Requests
	a.Errors += b.Errors
	a.ClientErrors += b.ClientErrors
	a.Sampled += b.Sampled
	a.DurationSum += b.DurationSum
	a.RequestBytes += b.RequestBytes
	a.ResponseBytes += b.ResponseBytes
	a.Durations.Merge(b.Durations)
	a.Sizes.Merge(b.Sizes)
}

func (a *Agg) Clone() Agg {
	out := *a
	out.Durations, out.Sizes = a.Durations.Clone(), a.Sizes.Clone()
	return out
}

// DurationQuantile is a duration percentile in microseconds, kept within the exact minimum and
// maximum.
func (a *Agg) DurationQuantile(q float64) float64 {
	if a.Requests == 0 {
		return 0
	}
	return min(max(a.Durations.Quantile(q), float64(a.DurationMin)), float64(a.DurationMax))
}

// CoreKey is a minute row's key, with the dictionaries' ids.
type CoreKey struct {
	Environment, Service, Route, APIVersion int32
	Method, Status                          int16
	Source                                  Source
	Platform                                Platform
}

// DimKey is an extra dimension row's key.
type DimKey struct {
	Dim         int16
	Value       int32
	Route       int32
	Method      int16
	StatusClass int16
}

// MinuteRow is a writer's cumulative totals of a key in a UTC minute; Hour and Day place the minute
// in the property's local hour and day.
type MinuteRow struct {
	WorkspaceID, PropertyID uuid.UUID
	Minute, Hour, Day       time.Time
	Writer                  int64
	Key                     CoreKey
	Agg                     Agg
}

type DimRow struct {
	WorkspaceID, PropertyID uuid.UUID
	Hour, Day               time.Time
	Writer                  int64
	Key                     DimKey
	Agg                     Agg
}

// Count is a number of records of a property's day for one reason.
type Count struct {
	WorkspaceID, PropertyID uuid.UUID
	Day                     time.Time
	Reason                  string
	N                       int64
}

// HourMark is a local hour a write touched; Capped says it reached its dimension rows.
type HourMark struct {
	WorkspaceID, PropertyID uuid.UUID
	Hour, Day               time.Time
	Capped                  bool
}

// Write is one flush: rows, the hours they touched, ingest counts and each workspace's new records.
type Write struct {
	Minutes []MinuteRow
	Dims    []DimRow
	Hours   []HourMark
	Counts  []Count
	Usage   map[uuid.UUID]int64
}

// NameRequest asks for the id of a value of a kind; Inferred applies to routes.
type NameRequest struct {
	Kind     string
	Value    string
	Inferred bool
}

type Store interface {
	// EnsureMinutePartitions makes the minute partitions of the UTC days from to to exist;
	// EnsureHourPartitions the hour partitions of the months of from to to.
	EnsureMinutePartitions(ctx context.Context, from, to time.Time) error
	EnsureHourPartitions(ctx context.Context, from, to time.Time) error
	// Names returns the ids of values of a property, adding the new ones as far as their caps
	// allow and giving the others (other)'s id.
	Names(ctx context.Context, workspaceID, propertyID uuid.UUID, reqs []NameRequest, now time.Time) ([]int32, error)
	// Write upserts a flush in one transaction.
	Write(ctx context.Context, w Write, now time.Time) error
	// Query answers one query over a property's rows.
	Query(ctx context.Context, q Query) (Result, error)
}
