package analytics

import (
	"context"
	"encoding/json"
	"time"
	"uuid"
)

// RawEvent is one stored event with its visit's description, as exports write it: nothing that
// names the visitor (no hash, contact or stored visit id); Visit is the stored id, which the
// exporter replaces by a number of its own.
type RawEvent struct {
	Time           time.Time       `json:"time"`
	Visit          int64           `json:"-"`
	Type           string          `json:"type"`
	Name           string          `json:"name,omitempty"`
	Hostname       string          `json:"hostname"`
	Path           string          `json:"path"`
	Props          json.RawMessage `json:"props,omitempty"`
	RevenueMinor   *int64          `json:"revenue_minor,omitempty"`
	Currency       *string         `json:"currency,omitempty"`
	EngagedMs      *int32          `json:"engaged_ms,omitempty"`
	Scroll         *int16          `json:"scroll,omitempty"`
	Channel        string          `json:"channel"`
	Source         string          `json:"source"`
	Referrer       string          `json:"referrer"`
	UTMSource      string          `json:"utm_source"`
	UTMMedium      string          `json:"utm_medium"`
	UTMCampaign    string          `json:"utm_campaign"`
	UTMTerm        string          `json:"utm_term"`
	UTMContent     string          `json:"utm_content"`
	Country        string          `json:"country"`
	Region         string          `json:"region"`
	City           string          `json:"city"`
	Device         string          `json:"device"`
	Browser        string          `json:"browser"`
	BrowserVersion string          `json:"browser_version"`
	OS             string          `json:"os"`
	OSVersion      string          `json:"os_version"`
	ScreenSize     string          `json:"screen_size"`
	Language       string          `json:"language"`
	Platform       string          `json:"platform"`
	AppVersion     string          `json:"app_version"`
}

var kindNames = map[int]string{int(KindPageview): "pageview", int(KindScreen): "screen", int(KindCustom): "event", int(KindEngagement): "engagement"}

func rawEventsSQL() string {
	dim := func(name string) string {
		d, _ := DimByName(name)
		return d.Group
	}
	return `SELECT e.ts, e.visit, ` + caseOf("e.kind", kindNames) + `, e.name, e.hostname, e.path, e.props, e.revenue_minor, e.currency,
	e.engaged_ms, e.scroll, ` + dim("channel") + `, v.source, v.referrer_host, v.utm_source, v.utm_medium, v.utm_campaign, v.utm_term,
	v.utm_content, v.country, v.region, v.city, ` + dim("device") + `, v.browser, CASE WHEN v.browser_major = 0 THEN '' ELSE v.browser_major::text END,
	v.os, CASE WHEN v.os_major = 0 THEN '' ELSE v.os_major::text END, ` + dim("screen_size") + `, v.language, ` + dim("platform") + `, v.app_version
FROM analytics_events e
JOIN analytics_visits v ON v.property_id = e.property_id AND v.day = e.day AND v.visit = e.visit AND v.workspace_id = e.workspace_id
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day = $3 AND NOT v.bot
ORDER BY e.ts, e.visit`
}

// RawEvents calls fn with every event of a property's day that is not a bot's, in time order.
func (s *PGStore) RawEvents(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, fn func(RawEvent) error) error {
	rows, err := s.Pool.Query(ctx, rawEventsSQL(), workspaceID, propertyID, day)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e RawEvent
		var props []byte
		if err := rows.Scan(&e.Time, &e.Visit, &e.Type, &e.Name, &e.Hostname, &e.Path, &props, &e.RevenueMinor, &e.Currency,
			&e.EngagedMs, &e.Scroll, &e.Channel, &e.Source, &e.Referrer, &e.UTMSource, &e.UTMMedium, &e.UTMCampaign, &e.UTMTerm,
			&e.UTMContent, &e.Country, &e.Region, &e.City, &e.Device, &e.Browser, &e.BrowserVersion, &e.OS, &e.OSVersion,
			&e.ScreenSize, &e.Language, &e.Platform, &e.AppVersion); err != nil {
			return err
		}
		if len(props) > 0 && string(props) != "null" {
			e.Props = props
		}
		e.Time = e.Time.UTC()
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
}
