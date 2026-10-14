package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"time"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
)

const maxAppRequestRecords = 100

var (
	limitAppRecordsPerAddress = limit{600, time.Minute}
	appMonitorPlatforms       = map[string]monitor.Platform{"ios": monitor.PlatformIOS, "android": monitor.PlatformAndroid}
)

// serveAppRequests takes an app's records of its calls to its own API for the API property
// analyticsCORS found by its public key. They follow the property's collection rules as app events
// do; the connecting address gives the country and the per-address limit, shortened, and is
// dropped.
func (s *Server) serveAppRequests(w http.ResponseWriter, r *http.Request) {
	cp, _ := r.Context().Value(analyticsPropertyKey).(*cachedProperty)
	m, err := s.monitorOn()
	if err != nil || cp == nil {
		writeProblem(w, errNotFound)
		return
	}
	if r.Header.Get("Sec-GPC") == "1" || r.Header.Get("DNT") == "1" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	ctx, now, prop := r.Context(), s.now(), cp.prop
	zone := monitorZone(prop.Timezone, prop.TimezoneHistory)
	count := func(reason string, n int) {
		if n > 0 {
			m.agg.Count(prop.WorkspaceID, prop.ID, zone.LocalDay(now), reason, int64(n))
		}
	}
	refuse := func(e *apiError) {
		count(monitor.ReasonInvalid, 1)
		writeProblem(w, e)
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAnalyticsBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			refuse(errAnalyticsBodyTooLarge)
		} else {
			refuse(errValidation("the body could not be read"))
		}
		return
	}
	var in struct {
		A       *oas.AnalyticsApp               `json:"a"`
		Records []oas.AnalyticsAppRequestRecord `json:"records"`
	}
	if json.Unmarshal(body, &in) != nil {
		refuse(errValidation(`apps send {"a": ..., "records": [...]}`))
		return
	}
	if in.A == nil {
		refuse(errValidation("a is required"))
		return
	}
	if len(in.Records) == 0 || len(in.Records) > maxAppRequestRecords {
		refuse(errValidation("records must hold 1 to 100 records"))
		return
	}
	platform, ok := appMonitorPlatforms[string(in.A.Platform)]
	if !ok {
		refuse(errValidation("a.platform must be ios or android"))
		return
	}
	if !slices.Contains(prop.Platforms, string(in.A.Platform)) {
		refuse(errPlatformNotAllowed)
		return
	}
	consent := prop.Collection == string(oas.AnalyticsCollectionConsent)
	version := analytics.Clean(derefString(in.A.Version), maxAppVersion)
	recs := make([]monitor.Record, 0, len(in.Records))
	invalid, late := 0, 0
	for _, ar := range in.Records {
		if consent && (ar.C == nil || ar.C.A == nil || *ar.C.A != 1) {
			continue
		}
		rr := oas.AnalyticsRequestRecord{
			Method: ar.Method, Route: ar.Route, Path: ar.Path, Status: ar.Status, DurationUs: ar.DurationUs,
			RequestSize: ar.RequestSize, ResponseSize: ar.ResponseSize, Time: ar.Time, ApiVersion: ar.ApiVersion, Environment: ar.Environment,
		}
		rec, isLate, err := s.requestRecord(&rr, zone, cp.rules, now)
		if err != nil {
			invalid++
			continue
		}
		if isLate {
			late++
			continue
		}
		rec.Source, rec.Platform, rec.AppVersion = monitor.SourceApp, platform, version
		recs = append(recs, rec)
	}
	count(monitor.ReasonInvalid, invalid)
	count(monitor.ReasonLate, late)
	if len(recs) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	addr, err := netip.ParseAddr(s.visitorIP(r))
	if err != nil {
		count(monitor.ReasonInvalid, len(recs))
		w.WriteHeader(http.StatusAccepted)
		return
	}
	short := analytics.ShortenAddr(addr.Unmap())
	addr = netip.Addr{}
	country := s.analytics.geo.Lookup(short).Country
	for i := range recs {
		if ok, wait := s.analytics.limits.allowWait("ma:"+prop.ID.String()+":"+short.String(), limitAppRecordsPerAddress, now); !ok {
			count(monitor.ReasonRate, len(recs)-i)
			writeProblem(w, rateLimited(wait))
			return
		}
		recs[i].Country = country
	}
	if _, err := s.admitRecords(ctx, m, prop, "", recs, now, count); err != nil {
		var e *apiError
		if !errors.As(err, &e) {
			s.log.ErrorContext(ctx, "app request records", slog.Any("error", err))
			e = errInternal
		}
		writeProblem(w, e)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
