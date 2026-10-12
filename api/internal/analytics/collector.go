package analytics

import (
	"context"
	"time"
	"uuid"
)

// Visitor is who an event belongs to: the day's visitor hash and their visit.
type Visitor struct {
	Day      time.Time
	Visitor  int64
	Visit    int64
	NewVisit bool
}

// Resolver turns a request's shortened address and user agent into the day's visitor and visit.
type Resolver struct {
	Store  Store
	Salts  *Salts
	Visits *Visits
	// Grace is how long after its day ends a salt is still accepted.
	Grace time.Duration
}

// Resolve finds the visitor's current visit, or starts one at now. An engagement never starts a
// visit: it joins the visitor's latest visit of the day, and ok is false when there is none.
func (r *Resolver) Resolve(ctx context.Context, workspaceID, propertyID uuid.UUID, loc *time.Location, now time.Time, ip ShortIP, userAgent string, engagement bool) (Visitor, bool, error) {
	day := LocalDay(now, loc)
	salt, err := r.Salts.Get(ctx, workspaceID, propertyID, day, DayEnd(day, loc).Add(r.Grace), now)
	if err != nil {
		return Visitor{}, false, err
	}
	out := Visitor{Day: day, Visitor: VisitorHash(salt, propertyID, ip, userAgent)}
	visit, last, cached := r.Visits.Get(propertyID, out.Visitor, day)
	if cached && (engagement || now.Sub(last) <= VisitGap) {
		out.Visit = visit
		if !engagement {
			r.Visits.Put(propertyID, out.Visitor, day, visit, now)
		}
		return out, true, nil
	}
	visit, ended, found, err := r.Store.LatestVisit(ctx, workspaceID, propertyID, day, out.Visitor)
	if err != nil {
		return Visitor{}, false, err
	}
	switch {
	case engagement && found:
		out.Visit = visit
		return out, true, nil
	case engagement:
		return out, false, nil
	case found && now.Sub(ended) <= VisitGap:
		out.Visit = visit
	default:
		out.Visit, out.NewVisit = VisitHash(salt, out.Visitor, now), true
	}
	r.Visits.Put(propertyID, out.Visitor, day, out.Visit, now)
	return out, true, nil
}
