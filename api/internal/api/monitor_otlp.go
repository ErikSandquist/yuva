package api

import (
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/otlp"
)

const otlpPrefix = "/otlp/"

var (
	errOTLPType     = problem(http.StatusUnsupportedMediaType, "unsupported_media_type", "send application/x-protobuf or application/json")
	errOTLPTooLarge = problem(http.StatusRequestEntityTooLarge, "payload_too_large", "send at most 8 MiB")
)

// otlpCode is the google.rpc.Status code an OTLP exporter reads from an answer's HTTP status.
func otlpCode(status int) int32 {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
		return otlp.CodeInvalidArgument
	case http.StatusUnauthorized:
		return otlp.CodeUnauthenticated
	case http.StatusForbidden:
		return otlp.CodePermissionDenied
	case http.StatusNotFound:
		return otlp.CodeNotFound
	case http.StatusUnprocessableEntity:
		return otlp.CodeFailedPrecondition
	case http.StatusTooManyRequests:
		return otlp.CodeResourceExhausted
	case http.StatusServiceUnavailable:
		return otlp.CodeUnavailable
	}
	return otlp.CodeInternal
}

func writeOTLP(w http.ResponseWriter, asJSON bool, status int, body []byte) {
	if asJSON {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "application/x-protobuf")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeOTLPError(w http.ResponseWriter, asJSON bool, e *apiError) {
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	msg, code := e.Code+": "+e.Detail, otlpCode(e.Status)
	if asJSON {
		writeOTLP(w, true, e.Status, otlp.StatusJSON(code, msg))
		return
	}
	writeOTLP(w, false, e.Status, otlp.StatusProto(code, msg))
}

// serveOTLPTraces takes an OTLP/HTTP trace export for an API property: its SERVER spans become
// request records of the server's source otlp.
func (s *Server) serveOTLPTraces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	asJSON := media != "application/x-protobuf" && media != "application/protobuf"
	fail := func(err error) {
		var e *apiError
		if !errors.As(err, &e) {
			s.log.ErrorContext(ctx, "otlp traces", slog.Any("error", err))
			e = errInternal
		}
		writeOTLPError(w, asJSON, e)
	}
	m, err := s.monitorOn()
	if err != nil {
		fail(err)
		return
	}
	if asJSON && media != "application/json" {
		fail(errOTLPType)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		fail(errUnauthenticated)
		return
	}
	p, err := s.resolveBearer(ctx, token, accessMemberOrKey, nil, false)
	if err != nil {
		fail(err)
		return
	}
	if !p.isKey() {
		fail(errMonitorNeedsKey)
		return
	}
	if err := requireScope(p, oas.AnalyticsWrite); err != nil {
		fail(err)
		return
	}
	id, err := uuid.Parse(r.PathValue("propertyId"))
	if err != nil {
		fail(errNotFound)
		return
	}
	prop, err := visibleProperty(ctx, s.st.Queries, p, id, false)
	if err != nil {
		fail(err)
		return
	}
	if !isAPIProperty(prop) {
		fail(errAPIOnly)
		return
	}
	body, err := readOTLPBody(w, r)
	if err != nil {
		fail(err)
		return
	}
	var spans []otlp.Span
	if asJSON {
		spans, err = otlp.DecodeJSON(body)
	} else {
		spans, err = otlp.DecodeProto(body)
	}
	if err != nil {
		fail(errValidation(err.Error()))
		return
	}
	now, zone := s.now(), monitorZone(prop.Timezone, prop.TimezoneHistory)
	count := func(reason string, n int) {
		if n > 0 {
			m.agg.Count(prop.WorkspaceID, prop.ID, zone.LocalDay(now), reason, int64(n))
		}
	}
	rules := pathRules(prop.PathRules)
	recs := make([]monitor.Record, 0, len(spans))
	invalid, late, reason := 0, 0, ""
	for i := range spans {
		sp := &spans[i]
		if sp.Method == "" {
			continue
		}
		rec, isLate, err := s.spanRecord(sp, zone, rules, now)
		if err != nil {
			if invalid++; reason == "" {
				reason = "span " + strconv.Itoa(i+1) + " of the SERVER spans: " + err.Error()
			}
			continue
		}
		if isLate {
			late++
			continue
		}
		recs = append(recs, rec)
	}
	count(monitor.ReasonInvalid, invalid)
	count(monitor.ReasonLate, late)
	wait, err := s.admitRecords(ctx, m, prop, keyRecordsLimit(p), recs, now, count)
	if err != nil {
		fail(err)
		return
	}
	if wait > 0 {
		fail(rateLimited(wait))
		return
	}
	if reason != "" {
		reason = strconv.Itoa(invalid) + " invalid; " + reason
	}
	if asJSON {
		writeOTLP(w, true, http.StatusOK, otlp.ResponseJSON(int64(invalid), reason))
		return
	}
	writeOTLP(w, false, http.StatusOK, otlp.ResponseProto(int64(invalid), reason))
}

// readOTLPBody reads the export, unpacking gzip, up to the server's body limit either way.
func readOTLPBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var src io.Reader = r.Body
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, errValidation("the body is not gzip")
		}
		defer zr.Close()
		src = http.MaxBytesReader(w, zr, maxBodyBytes)
	default:
		return nil, errEncoding
	}
	body, err := io.ReadAll(src)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errOTLPTooLarge
		}
		return nil, errValidation("the body could not be read")
	}
	return body, nil
}

// spanRecord checks a SERVER span as the request record it stands for, weighed by its sampling
// threshold. Unlike a sampled record, a sampled span may carry an error: head sampling decides
// before the response.
func (s *Server) spanRecord(sp *otlp.Span, zone monitor.Zone, rules []analytics.PathRule, now time.Time) (monitor.Record, bool, error) {
	if sp.End < sp.Start || sp.Start > math.MaxInt64 {
		return monitor.Record{}, false, errors.New("the span ends before it starts")
	}
	opt := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	size := func(v int64) *int64 {
		if v < 0 {
			return nil
		}
		return &v
	}
	start := time.Unix(0, int64(sp.Start)).UTC()
	in := oas.AnalyticsRequestRecord{
		Method: sp.Method, Route: opt(sp.Route), Path: opt(sp.Path), Status: int(min(max(sp.Status, -1), 1000)),
		DurationUs: int64((sp.End - sp.Start) / 1000), RequestSize: size(sp.RequestSize), ResponseSize: size(sp.ResponseSize), Time: &start,
		Consumer: opt(sp.Consumer), ApiVersion: opt(sp.APIVersion), Ip: opt(sp.ClientAddress),
		Environment: opt(sp.Resource.Environment), Service: opt(sp.Resource.Service), Release: opt(sp.Resource.Version),
	}
	if sp.Status < 0 {
		in.Status = 0
	}
	rec, late, err := s.requestRecord(&in, zone, rules, now)
	if err != nil {
		return rec, false, err
	}
	weight := math.Round(otlp.AdjustedCount(sp.TraceState))
	if weight > maxSampleRate {
		return rec, false, errors.New("the tracestate threshold samples fewer than 1 in 10,000 spans")
	}
	rec.Source, rec.Weight = monitor.SourceOTLP, int64(weight)
	return rec, late, nil
}
