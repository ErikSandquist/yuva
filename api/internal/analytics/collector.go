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
	// Bot is a visitor over the day's volume limits; NewBot is true for the event that made it one,
	// whose earlier visits of the day are then marked too.
	Bot, NewBot bool
}

// Resolver turns a request's shortened address and user agent into the day's visitor and visit.
type Resolver struct {
	Store  Store
	Salts  *Salts
	Visits *Visits
	// Grace is how long after its day ends a salt is still accepted.
	Grace time.Duration
}

// Resolve finds the visitor's current visit, or starts one at now, and counts the event against the
// visitor's volume limits. An engagement never starts a visit: it joins the visitor's latest visit of
// the day, and ok is false when there is none.
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
		out.Bot, out.NewBot = r.Visits.Tally(propertyID, out.Visitor, day, now, !engagement)
		return out, true, nil
	}
	latest, found, err := r.Store.LatestVisit(ctx, workspaceID, propertyID, day, out.Visitor)
	if err != nil {
		return Visitor{}, false, err
	}
	switch {
	case engagement && found:
		out.Visit, out.Bot = latest.Visit, latest.Bot
		return out, true, nil
	case engagement:
		return out, false, nil
	case found && now.Sub(latest.Ended) <= VisitGap:
		out.Visit = latest.Visit
	default:
		out.Visit, out.NewVisit = VisitHash(salt, out.Visitor, now), true
	}
	r.Visits.Put(propertyID, out.Visitor, day, out.Visit, now)
	if found && latest.Bot {
		r.Visits.MarkBot(propertyID, out.Visitor, day)
	}
	out.Bot, out.NewBot = r.Visits.Tally(propertyID, out.Visitor, day, now, true)
	return out, true, nil
}
