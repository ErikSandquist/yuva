package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const noticeMonitorLimit = "monitor_limit"

var errRouteGone = problem(http.StatusNotFound, "not_found", "no such route")

func zoneHistory(raw []byte) []monitor.ZoneChange {
	var out []monitor.ZoneChange
	_ = json.Unmarshal(raw, &out)
	return out
}

// monitorZone is the zones an API property counted its hours and days in.
func monitorZone(tz string, history []byte) monitor.Zone {
	return monitor.NewZone(propertyLocation(tz), zoneHistory(history))
}

// nextZoneHistory is an API property's zone history after its zone changes from cur to the zone
// it is updated to: the zone before keeps counting until its next local midnight, unless an earlier
// change has not taken effect yet, which the new zone then replaces.
func nextZoneHistory(cur store.AnalyticsProperty, now time.Time) []byte {
	history := zoneHistory(cur.TimezoneHistory)
	if n := len(history); n > 0 && now.Before(history[n-1].Until) {
		return cur.TimezoneHistory
	}
	loc := propertyLocation(cur.Timezone)
	history = append(history, monitor.ZoneChange{Timezone: cur.Timezone, Until: monitor.DayStart(monitor.LocalDay(now, loc).AddDate(0, 0, 1), loc)})
	return mustJSON(history)
}

// zoneAppliesAt is when an API property's zone takes effect, while a change has not yet.
func zoneAppliesAt(prop store.AnalyticsProperty, now time.Time) *time.Time {
	history := zoneHistory(prop.TimezoneHistory)
	if n := len(history); isAPIProperty(prop) && n > 0 && now.Before(history[n-1].Until) {
		return &history[n-1].Until
	}
	return nil
}

func routeBody(r store.ListMonitorRoutesRow) oas.AnalyticsRoute {
	return oas.AnalyticsRoute{Id: r.ID, Pattern: r.Pattern, Inferred: r.Inferred, Hidden: r.Hidden, FirstSeenAt: r.FirstSeenAt, LastSeenAt: r.LastSeenAt}
}

func (s *Server) ListAnalyticsRoutes(ctx context.Context, req oas.ListAnalyticsRoutesRequestObject) (oas.ListAnalyticsRoutesResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	if !isAPIProperty(prop) {
		return nil, errAPIOnly
	}
	rows, err := s.st.ListMonitorRoutes(ctx, store.ListMonitorRoutesParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
	if err != nil {
		return nil, err
	}
	out := oas.AnalyticsRouteList{Items: make([]oas.AnalyticsRoute, len(rows))}
	for i, r := range rows {
		out.Items[i] = routeBody(r)
	}
	return oas.ListAnalyticsRoutes200JSONResponse(out), nil
}

func (s *Server) UpdateAnalyticsRoute(ctx context.Context, req oas.UpdateAnalyticsRouteRequestObject) (oas.UpdateAnalyticsRouteResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	var out store.SetMonitorRouteHiddenRow
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		if !isAPIProperty(prop) {
			return errAPIOnly
		}
		out, err = q.SetMonitorRouteHidden(ctx, store.SetMonitorRouteHiddenParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.RouteId,
			Hidden: req.Body.Hidden})
		if store.IsNotFound(err) {
			return errRouteGone
		}
		if err != nil {
			return err
		}
		if _, err := q.BumpAnalyticsRollupVersion(ctx, store.BumpAnalyticsRollupVersionParams{WorkspaceID: p.workspaceID, ID: prop.ID}); err != nil {
			return err
		}
		return notifyAnalyticsChanged(ctx, q, p.workspaceID, &prop.ID)
	})
	if err != nil {
		return nil, err
	}
	s.analytics.answers.forget(req.PropertyId)
	return oas.UpdateAnalyticsRoute200JSONResponse(routeBody(store.ListMonitorRoutesRow(out))), nil
}

// afterMonitorFlush tells owners and the operator, once a month, that a workspace's request
// records reached their cap.
func (s *Server) afterMonitorFlush(ctx context.Context, month time.Time, usage map[uuid.UUID]int64) {
	now := s.now()
	for ws, total := range usage {
		e, err := s.loadMonitorUsage(ctx, ws, now)
		if err != nil {
			s.log.ErrorContext(ctx, "monitor usage", slog.Any("error", err))
			continue
		}
		c := s.analytics.monitor.usage
		c.mu.Lock()
		if e.month.Equal(month) && total > e.events {
			e.events = total
		}
		limit, reached := e.limit, e.reached
		c.mu.Unlock()
		if limit <= 0 || total < limit || reached {
			continue
		}
		if err := s.noteMonitorLimit(ctx, ws, month, total, limit); err != nil {
			s.log.ErrorContext(ctx, "monitor limit notice", slog.Any("error", err))
		}
	}
}

func (s *Server) noteMonitorLimit(ctx context.Context, ws uuid.UUID, month time.Time, total, limit int64) error {
	var w store.Workspace
	var owners []store.ListOwnerAddressesRow
	first := false
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		n, err := q.MarkMonitorLimitReached(ctx, store.MarkMonitorLimitReachedParams{WorkspaceID: ws, Month: month, Now: new(s.now())})
		if err != nil || n == 0 {
			return err
		}
		first = true
		if w, err = q.GetWorkspace(ctx, ws); err != nil {
			return err
		}
		if owners, err = q.ListOwnerAddresses(ctx, ws); err != nil {
			return err
		}
		if len(s.operator) == 0 {
			return nil
		}
		return q.CreateOperatorNotice(ctx, store.CreateOperatorNoticeParams{
			ID: uuid.New(), Kind: noticeMonitorLimit, WorkspaceName: &w.Name, CreatedAt: s.now(),
			Detail: new(formatCount(total) + " API request records in " + month.Format("January 2006") + "; the limit is " + formatCount(limit)),
		})
	})
	c := s.analytics.monitor.usage
	c.mu.Lock()
	if e, ok := c.m[ws]; ok {
		e.reached = true
	}
	c.mu.Unlock()
	if err != nil || !first {
		return err
	}
	for _, o := range owners {
		msg, err := mail.Render("monitor_limit", o.Locale, map[string]any{
			"Workspace": w.Name, "Limit": formatCount(limit), "Month": month.Format("2006-01"), "URL": s.auth.PublicURL + "/analytics",
		})
		if err != nil {
			return err
		}
		msg.To = o.Email
		s.sendMail(ctx, msg)
	}
	return nil
}
