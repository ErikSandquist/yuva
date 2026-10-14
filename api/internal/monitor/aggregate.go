package monitor

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
	"uuid"
)

const (
	FlushInterval = 10 * time.Second
	// LastFlush is how long after its minute or hour ends a bucket is written a last time and
	// leaves memory; records for it after that are late.
	LastFlush = 2 * time.Minute
	// FoldAfter is how long after it ends an hour is folded. It outlasts every process's last
	// flush of the hour, so no writer still replaces its rows once they are compacted.
	FoldAfter    = 5 * time.Minute
	drainTimeout = 10 * time.Second
	maxBackoff   = time.Minute
	namesTTL     = 10 * time.Minute
	maxNames     = 2_000_000
)

// ErrFull: the aggregator holds as many open rows as it may.
var ErrFull = errors.New("monitor: too many open rows")

type coreCellKey struct {
	ws, prop uuid.UUID
	minute   time.Time
	key      CoreKey
}

type dimCellKey struct {
	ws, prop uuid.UUID
	hour     time.Time
	key      DimKey
}

type hourKey struct {
	prop uuid.UUID
	hour time.Time
}

type cell struct {
	hour, day time.Time
	agg       Agg
	dirty     bool
	flushed   bool
	final     time.Time
}

type countKey struct {
	ws, prop uuid.UUID
	day      time.Time
	reason   string
}

// Aggregator adds records up in memory, per UTC minute for the core key and per local hour for
// the extra dimensions, and writes each bucket's cumulative totals under its own writer id every
// ten seconds, replacing what it wrote before. A bucket is written a last time LastFlush after it
// ends and then forgotten; records for a bucket already gone are written once under a fresh
// writer.
type Aggregator struct {
	Store   Store
	Log     *slog.Logger
	Now     func() time.Time
	MaxKeys int
	// AfterFlush learns each workspace's request records of the month after a flush wrote some.
	AfterFlush func(ctx context.Context, month time.Time, usage map[uuid.UUID]int64)

	mu       sync.Mutex
	writer   int64
	core     map[coreCellKey]*cell
	dims     map[dimCellKey]*cell
	lateCore map[coreCellKey]*cell
	lateDims map[dimCellKey]*cell
	dimRows  map[hourKey]int
	capped   map[hourKey]bool
	counts   map[countKey]int64
	usage    map[uuid.UUID]int64

	names     nameCache
	partMu    sync.Mutex
	minuteDay map[time.Time]bool
	hourMonth map[time.Time]bool
}

func (a *Aggregator) init() {
	if a.core == nil {
		a.writer = newWriter()
		a.core, a.dims = map[coreCellKey]*cell{}, map[dimCellKey]*cell{}
		a.lateCore, a.lateDims = map[coreCellKey]*cell{}, map[dimCellKey]*cell{}
		a.dimRows, a.capped = map[hourKey]int{}, map[hourKey]bool{}
		a.counts, a.usage = map[countKey]int64{}, map[uuid.UUID]int64{}
	}
}

func newWriter() int64 { return rand.Int64N(1<<62) + 1 }

// Keys is how many rows the aggregator holds open.
func (a *Aggregator) Keys() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.core) + len(a.dims) + len(a.lateCore) + len(a.lateDims)
}

// Count records n records of a property's day that were not counted, or with ReasonAccepted
// adds to the accepted ones.
func (a *Aggregator) Count(workspaceID, propertyID uuid.UUID, day time.Time, reason string, n int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.init()
	a.counts[countKey{workspaceID, propertyID, day, reason}] += n
}

type resolved struct {
	core CoreKey
	dims [3]DimKey
	n    int
}

// Add takes all of recs or, with ErrFull, none. Their values get their dictionary ids first, so a
// value past its cap is counted as (other) from the start.
func (a *Aggregator) Add(ctx context.Context, p Property, recs []Record) error {
	if len(recs) == 0 {
		return nil
	}
	now := a.Now()
	var reqs []NameRequest
	seen := map[NameRequest]int{}
	ask := func(kind, value string, inferred bool) int {
		if value == "" {
			return -1
		}
		k := NameRequest{Kind: kind, Value: value}
		if i, ok := seen[k]; ok {
			return i
		}
		seen[k] = len(reqs)
		reqs = append(reqs, NameRequest{Kind: kind, Value: value, Inferred: inferred})
		return len(reqs) - 1
	}
	type refs struct{ env, svc, route, ver, consumer, country, app int }
	idx := make([]refs, len(recs))
	for i := range recs {
		r := &recs[i]
		idx[i] = refs{ask(KindEnvironment, r.Environment, false), ask(KindService, r.Service, false), ask(KindRoute, r.Route, r.Inferred),
			ask(KindAPIVersion, r.APIVersion, false), ask(KindConsumer, r.Consumer, false), ask(KindCountry, r.Country, false),
			ask(KindAppVersion, r.AppVersion, false)}
	}
	for _, kind := range []string{KindConsumer, KindCountry, KindAppVersion} {
		ask(kind, OtherValue, false)
	}
	ids, err := a.names.resolve(ctx, a.Store, p, reqs, now)
	if err != nil {
		return err
	}
	id := func(i int) int32 {
		if i < 0 {
			return 0
		}
		return ids[i]
	}
	other := map[int16]int32{}
	for d, kind := range dimKinds {
		other[d] = ids[seen[NameRequest{Kind: kind, Value: OtherValue}]]
	}
	keys := make([]resolved, len(recs))
	for i := range recs {
		r, x := &recs[i], idx[i]
		k := resolved{core: CoreKey{Environment: id(x.env), Service: id(x.svc), Route: id(x.route), APIVersion: id(x.ver),
			Method: r.Method, Status: r.Status, Source: r.Source, Platform: r.Platform}}
		for _, d := range []struct {
			dim int16
			ref int
		}{{DimConsumer, x.consumer}, {DimCountry, x.country}, {DimAppVersion, x.app}} {
			if d.ref >= 0 {
				k.dims[k.n] = DimKey{Dim: d.dim, Value: id(d.ref), Route: k.core.Route, Method: r.Method, StatusClass: StatusClass(r.Status)}
				k.n++
			}
		}
		keys[i] = k
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.init()
	fresh := 0
	newCore, newDims := map[coreCellKey]bool{}, map[dimCellKey]bool{}
	for i := range recs {
		r := &recs[i]
		minute := r.Time.UTC().Truncate(time.Minute)
		ck := coreCellKey{p.WorkspaceID, p.ID, minute, keys[i].core}
		if late := !minute.Add(time.Minute + LastFlush).After(now); late {
			if a.lateCore[ck] == nil && !newCore[ck] {
				newCore[ck], fresh = true, fresh+1
			}
		} else if a.core[ck] == nil && !newCore[ck] {
			newCore[ck], fresh = true, fresh+1
		}
		hour := r.Hour.UTC()
		for _, dk := range keys[i].dims[:keys[i].n] {
			k := dimCellKey{p.WorkspaceID, p.ID, hour, dk}
			if a.dims[k] == nil && a.lateDims[k] == nil && !newDims[k] {
				newDims[k], fresh = true, fresh+1
			}
		}
	}
	if a.MaxKeys > 0 && len(a.core)+len(a.dims)+len(a.lateCore)+len(a.lateDims)+fresh > a.MaxKeys {
		return ErrFull
	}
	for i := range recs {
		r := &recs[i]
		minute := r.Time.UTC().Truncate(time.Minute)
		hour, day := r.Hour.UTC(), r.Day
		ck := coreCellKey{p.WorkspaceID, p.ID, minute, keys[i].core}
		if !minute.Add(time.Minute + LastFlush).After(now) {
			add(a.lateCore, ck, hour, day, time.Time{}, r)
		} else {
			add(a.core, ck, hour, day, minute.Add(time.Minute+LastFlush), r)
		}
		hk := hourKey{p.ID, hour}
		late := !hour.Add(time.Hour + LastFlush).After(now)
		for _, dk := range keys[i].dims[:keys[i].n] {
			k := dimCellKey{p.WorkspaceID, p.ID, hour, dk}
			if late {
				add(a.lateDims, k, hour, day, time.Time{}, r)
				continue
			}
			if a.dims[k] == nil {
				if a.dimRows[hk] >= DimRowsPerHour {
					k.key.Value = other[dk.Dim]
					a.capped[hk] = true
				}
				if a.dims[k] == nil {
					a.dimRows[hk]++
				}
			}
			add(a.dims, k, hour, day, hour.Add(time.Hour+LastFlush), r)
		}
		a.usage[p.WorkspaceID]++
	}
	return nil
}

func add[K comparable](m map[K]*cell, k K, hour, day, final time.Time, r *Record) {
	c := m[k]
	if c == nil {
		c = &cell{hour: hour, day: day, agg: NewAgg(), final: final}
		m[k] = c
	}
	c.agg.Add(r)
	c.dirty = true
}

// pending is what one flush took from memory, to put back when the write fails.
type pending struct {
	w         Write
	core      []coreCellKey
	dims      []dimCellKey
	lateCore  map[coreCellKey]*cell
	lateDims  map[dimCellKey]*cell
	counts    map[countKey]int64
	usage     map[uuid.UUID]int64
	minuteDay []time.Time
	hourMonth []time.Time
}

func (a *Aggregator) take(now time.Time) *pending {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.init()
	p := &pending{lateCore: a.lateCore, lateDims: a.lateDims, counts: a.counts, usage: a.usage}
	a.lateCore, a.lateDims, a.counts, a.usage = map[coreCellKey]*cell{}, map[dimCellKey]*cell{}, map[countKey]int64{}, map[uuid.UUID]int64{}
	hours := map[hourKey]*HourMark{}
	mark := func(ws, prop uuid.UUID, hour, day time.Time) {
		k := hourKey{prop, hour}
		if hours[k] == nil {
			hours[k] = &HourMark{WorkspaceID: ws, PropertyID: prop, Hour: hour, Day: day, Capped: a.capped[k]}
		}
	}
	days, months := map[time.Time]bool{}, map[time.Time]bool{}
	for k, c := range a.core {
		if !c.dirty {
			continue
		}
		c.dirty = false
		p.core = append(p.core, k)
		p.w.Minutes = append(p.w.Minutes, MinuteRow{WorkspaceID: k.ws, PropertyID: k.prop, Minute: k.minute, Hour: c.hour, Day: c.day, Writer: a.writer, Key: k.key, Agg: c.agg.Clone()})
		mark(k.ws, k.prop, c.hour, c.day)
		days[utcDay(k.minute)] = true
	}
	for k, c := range a.dims {
		if !c.dirty {
			continue
		}
		c.dirty = false
		if c.flushed && !now.Before(k.hour.Add(time.Hour+FoldAfter)) {
			a.Log.Error("monitor: an hour's dimension rows were folded before this process wrote them a last time; its records since are lost",
				slog.String("property_id", k.prop.String()), slog.Time("hour", k.hour))
			delete(a.dims, k)
			continue
		}
		p.dims = append(p.dims, k)
		p.w.Dims = append(p.w.Dims, DimRow{WorkspaceID: k.ws, PropertyID: k.prop, Hour: k.hour, Day: c.day, Writer: a.writer, Key: k.key, Agg: c.agg.Clone()})
		mark(k.ws, k.prop, c.hour, c.day)
		months[monthOf(k.hour)] = true
	}
	if len(p.lateCore)+len(p.lateDims) > 0 {
		late := newWriter()
		for k, c := range p.lateCore {
			p.w.Minutes = append(p.w.Minutes, MinuteRow{WorkspaceID: k.ws, PropertyID: k.prop, Minute: k.minute, Hour: c.hour, Day: c.day, Writer: late, Key: k.key, Agg: c.agg})
			mark(k.ws, k.prop, c.hour, c.day)
			days[utcDay(k.minute)] = true
		}
		for k, c := range p.lateDims {
			p.w.Dims = append(p.w.Dims, DimRow{WorkspaceID: k.ws, PropertyID: k.prop, Hour: k.hour, Day: c.day, Writer: late, Key: k.key, Agg: c.agg})
			mark(k.ws, k.prop, c.hour, c.day)
			months[monthOf(k.hour)] = true
		}
	}
	for _, h := range hours {
		p.w.Hours = append(p.w.Hours, *h)
	}
	slices.SortFunc(p.w.Hours, func(x, y HourMark) int { return cmp.Or(x.PropertyID.Compare(y.PropertyID), x.Hour.Compare(y.Hour)) })
	for k, n := range p.counts {
		p.w.Counts = append(p.w.Counts, Count{WorkspaceID: k.ws, PropertyID: k.prop, Day: k.day, Reason: k.reason, N: n})
	}
	p.w.Usage = maps.Clone(p.usage)
	p.minuteDay, p.hourMonth = slices.Collect(maps.Keys(days)), slices.Collect(maps.Keys(months))
	return p
}

func (p *pending) empty() bool {
	return len(p.w.Minutes) == 0 && len(p.w.Dims) == 0 && len(p.w.Counts) == 0 && len(p.w.Usage) == 0
}

func (a *Aggregator) putBack(p *pending) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, k := range p.core {
		if c := a.core[k]; c != nil {
			c.dirty = true
		}
	}
	for _, k := range p.dims {
		if c := a.dims[k]; c != nil {
			c.dirty = true
		}
	}
	for k, c := range p.lateCore {
		if cur := a.lateCore[k]; cur != nil {
			cur.agg.Merge(&c.agg)
		} else {
			a.lateCore[k] = c
		}
	}
	for k, c := range p.lateDims {
		if cur := a.lateDims[k]; cur != nil {
			cur.agg.Merge(&c.agg)
		} else {
			a.lateDims[k] = c
		}
	}
	for k, n := range p.counts {
		a.counts[k] += n
	}
	for k, n := range p.usage {
		a.usage[k] += n
	}
}

// forget drops the buckets that were written a last time and have not changed since.
func (a *Aggregator) forget(p *pending, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, k := range p.core {
		if c := a.core[k]; c != nil {
			c.flushed = true
		}
	}
	for _, k := range p.dims {
		if c := a.dims[k]; c != nil {
			c.flushed = true
		}
	}
	for k, c := range a.core {
		if !c.dirty && !now.Before(c.final) {
			delete(a.core, k)
		}
	}
	for k, c := range a.dims {
		if !c.dirty && !now.Before(c.final) {
			delete(a.dims, k)
		}
	}
	for k := range a.dimRows {
		if !now.Before(k.hour.Add(time.Hour + LastFlush)) {
			delete(a.dimRows, k)
			delete(a.capped, k)
		}
	}
}

// Prepare makes the minute partitions of yesterday to two days ahead (UTC) and the hour partitions
// of this month and the next exist.
func (a *Aggregator) Prepare(ctx context.Context) error {
	today := utcDay(a.Now())
	if err := a.ensure(ctx, []time.Time{today.AddDate(0, 0, -1), today, today.AddDate(0, 0, 1), today.AddDate(0, 0, 2)}, nil); err != nil {
		return err
	}
	m := monthOf(today)
	return a.ensure(ctx, nil, []time.Time{m.AddDate(0, -1, 0), m, m.AddDate(0, 1, 0)})
}

func (a *Aggregator) ensure(ctx context.Context, days, months []time.Time) error {
	a.partMu.Lock()
	defer a.partMu.Unlock()
	if a.minuteDay == nil {
		a.minuteDay, a.hourMonth = map[time.Time]bool{}, map[time.Time]bool{}
	}
	for _, d := range days {
		if !a.minuteDay[d] {
			if err := a.Store.EnsureMinutePartitions(ctx, d, d); err != nil {
				return err
			}
			a.minuteDay[d] = true
		}
	}
	for _, m := range months {
		if !a.hourMonth[m] {
			if err := a.Store.EnsureHourPartitions(ctx, m, m); err != nil {
				return err
			}
			a.hourMonth[m] = true
		}
	}
	return nil
}

// Flush writes every bucket that changed since the last flush; on failure everything stays for the
// next one.
func (a *Aggregator) Flush(ctx context.Context) error {
	now := a.Now()
	p := a.take(now)
	if p.empty() {
		a.forget(p, now)
		return nil
	}
	err := a.ensure(ctx, p.minuteDay, p.hourMonth)
	var totals map[uuid.UUID]int64
	if err == nil {
		totals, err = a.Store.Write(ctx, p.w, now)
	}
	if err != nil {
		a.putBack(p)
		return err
	}
	a.forget(p, now)
	if a.AfterFlush != nil && len(totals) > 0 {
		a.AfterFlush(ctx, monthOf(now), totals)
	}
	return nil
}

// Run flushes every ten seconds until ctx ends, then writes everything left for up to ten seconds.
func (a *Aggregator) Run(ctx context.Context) {
	tick := time.NewTicker(FlushInterval)
	defer tick.Stop()
	backoff := time.Duration(0)
	var retryAt time.Time
	for {
		select {
		case <-ctx.Done():
			a.drain()
			return
		case <-tick.C:
		}
		if time.Now().Before(retryAt) {
			continue
		}
		if err := a.Flush(ctx); err != nil {
			backoff = min(max(backoff*2, FlushInterval), maxBackoff)
			retryAt = time.Now().Add(backoff)
			a.Log.ErrorContext(ctx, "monitor flush", slog.Any("error", err), slog.Duration("retry_in", backoff))
			continue
		}
		backoff = 0
	}
}

func (a *Aggregator) drain() {
	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	for ctx.Err() == nil {
		err := a.Flush(ctx)
		if err == nil {
			return
		}
		a.Log.Error("monitor drain", slog.Any("error", err), slog.Int("rows_left", a.Keys()))
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
}

// Forget drops what the aggregator knows of a property's names, after it was deleted or changed.
func (a *Aggregator) Forget(propertyID uuid.UUID) { a.names.forget(propertyID) }

type nameKey struct {
	kind, value string
}

type propNames struct {
	ids    map[nameKey]int32
	full   map[string]bool
	loaded time.Time
}

// nameCache keeps a property's dictionary ids for ten minutes. A kind whose cap is reached maps
// every value it does not know to (other) without asking again.
type nameCache struct {
	mu    sync.Mutex
	props map[uuid.UUID]*propNames
	size  int
}

func (c *nameCache) forget(propertyID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pn := c.props[propertyID]; pn != nil {
		c.size -= len(pn.ids)
		delete(c.props, propertyID)
	}
}

func (c *nameCache) resolve(ctx context.Context, st Store, p Property, reqs []NameRequest, now time.Time) ([]int32, error) {
	out := make([]int32, len(reqs))
	var miss []int
	c.mu.Lock()
	if c.props == nil || c.size > maxNames {
		c.props, c.size = map[uuid.UUID]*propNames{}, 0
	}
	pn := c.props[p.ID]
	if pn == nil || now.Sub(pn.loaded) > namesTTL || now.Before(pn.loaded) {
		if pn != nil {
			c.size -= len(pn.ids)
		}
		pn = &propNames{ids: map[nameKey]int32{}, full: map[string]bool{}, loaded: now}
		c.props[p.ID] = pn
	}
	for i, r := range reqs {
		if id, ok := pn.ids[nameKey{r.Kind, r.Value}]; ok {
			out[i] = id
			continue
		}
		if other, ok := pn.ids[nameKey{r.Kind, OtherValue}]; ok && pn.full[r.Kind] {
			out[i] = other
			continue
		}
		miss = append(miss, i)
	}
	c.mu.Unlock()
	if len(miss) == 0 {
		return out, nil
	}
	ask := make([]NameRequest, 0, len(miss)+1)
	kinds := map[string]bool{}
	for _, i := range miss {
		ask = append(ask, reqs[i])
		kinds[reqs[i].Kind] = true
	}
	others := map[string]int{}
	for _, kind := range slices.Sorted(maps.Keys(kinds)) {
		if _, capped := Caps[kind]; capped {
			others[kind] = len(ask)
			ask = append(ask, NameRequest{Kind: kind, Value: OtherValue})
		}
	}
	ids, err := st.Names(ctx, p.WorkspaceID, p.ID, ask, now)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for kind, at := range others {
		if _, ok := pn.ids[nameKey{kind, OtherValue}]; !ok {
			c.size++
		}
		pn.ids[nameKey{kind, OtherValue}] = ids[at]
	}
	for j, i := range miss {
		r := reqs[i]
		out[i] = ids[j]
		if at, ok := others[r.Kind]; ok && ids[j] == ids[at] && r.Value != OtherValue {
			pn.full[r.Kind] = true
			continue
		}
		if _, ok := pn.ids[nameKey{r.Kind, r.Value}]; !ok {
			c.size++
		}
		pn.ids[nameKey{r.Kind, r.Value}] = ids[j]
	}
	return out, nil
}

func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func monthOf(t time.Time) time.Time {
	y, m, _ := t.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}
