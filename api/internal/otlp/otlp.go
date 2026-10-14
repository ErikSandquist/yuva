// Package otlp reads the few fields API monitoring needs from OTLP/HTTP trace exports, in binary
// protobuf with protowire and in OTLP/JSON, and writes the export answers, without the OTLP
// modules.
package otlp

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// KindServer is SPAN_KIND_SERVER.
const KindServer = 2

// Resource is what a span's resource says about the service that sent it.
type Resource struct {
	Service     string
	Environment string
	Version     string
	Instance    string
}

// Span is a SERVER span's fields that make a request record. Status and the sizes are -1 when the
// span does not carry them.
type Span struct {
	Resource      *Resource
	TraceState    string
	Start, End    uint64
	Method        string
	Route         string
	Path          string
	Status        int64
	ClientAddress string
	Consumer      string
	APIVersion    string
	RequestSize   int64
	ResponseSize  int64
}

func newSpan(res *Resource) Span {
	return Span{Resource: res, Status: -1, RequestSize: -1, ResponseSize: -1}
}

// value is an attribute's AnyValue of a kind the fields above can use.
type value struct {
	str    string
	isStr  bool
	num    int64
	isNum  bool
	double float64
	isDbl  bool
}

func (v value) text() string {
	switch {
	case v.isStr:
		return v.str
	case v.isNum:
		return strconv.FormatInt(v.num, 10)
	}
	return ""
}

func (v value) integer() (int64, bool) {
	switch {
	case v.isNum:
		return v.num, true
	case v.isDbl && v.double == math.Trunc(v.double) && math.Abs(v.double) < 1<<53:
		return int64(v.double), true
	case v.isStr:
		n, err := strconv.ParseInt(strings.TrimSpace(v.str), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func resourceAttr(res *Resource, key string, v value) {
	switch key {
	case "service.name":
		res.Service = v.text()
	case "deployment.environment.name":
		res.Environment = v.text()
	case "deployment.environment":
		if res.Environment == "" {
			res.Environment = v.text()
		}
	case "service.version":
		res.Version = v.text()
	case "service.instance.id":
		res.Instance = v.text()
	}
}

// spanKeys are the span attributes read; every other attribute is skipped undecoded.
var spanKeys = map[string]bool{
	"http.request.method": true, "http.route": true, "url.path": true, "http.response.status_code": true, "client.address": true,
	"http.request.body.size": true, "http.response.body.size": true, "yuva.consumer": true, "yuva.api_version": true,
}

func spanAttr(s *Span, key string, v value) {
	size := func(dst *int64) {
		if n, ok := v.integer(); ok {
			*dst = n
		}
	}
	switch key {
	case "http.request.method":
		s.Method = v.text()
	case "http.route":
		s.Route = v.text()
	case "url.path":
		s.Path = v.text()
	case "http.response.status_code":
		size(&s.Status)
	case "client.address":
		s.ClientAddress = v.text()
	case "http.request.body.size":
		size(&s.RequestSize)
	case "http.response.body.size":
		size(&s.ResponseSize)
	case "yuva.consumer":
		s.Consumer = v.text()
	case "yuva.api_version":
		s.APIVersion = v.text()
	}
}

// AdjustedCount is how many spans a span stands for under OpenTelemetry probability sampling:
// 2^56 / (2^56 - T) for the rejection threshold T of its tracestate's ot=th, and 1 without one.
func AdjustedCount(traceState string) float64 {
	for member := range strings.SplitSeq(traceState, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(member), "=")
		if !ok || k != "ot" {
			continue
		}
		for sub := range strings.SplitSeq(v, ";") {
			sk, sv, ok := strings.Cut(sub, ":")
			if !ok || sk != "th" {
				continue
			}
			if sv == "" || len(sv) > 14 {
				return 1
			}
			t, err := strconv.ParseUint(sv+strings.Repeat("0", 14-len(sv)), 16, 64)
			if err != nil {
				return 1
			}
			return float64(uint64(1)<<56) / float64(uint64(1)<<56-t)
		}
	}
	return 1
}

// Status codes of google.rpc.Status used in error answers.
const (
	CodeInvalidArgument    int32 = 3
	CodeNotFound           int32 = 5
	CodePermissionDenied   int32 = 7
	CodeResourceExhausted  int32 = 8
	CodeFailedPrecondition int32 = 9
	CodeInternal           int32 = 13
	CodeUnavailable        int32 = 14
	CodeUnauthenticated    int32 = 16
)

// ResponseJSON is an ExportTraceServiceResponse in OTLP/JSON; partial_success is set only when
// spans were rejected.
func ResponseJSON(rejected int64, message string) []byte {
	if rejected == 0 && message == "" {
		return []byte("{}")
	}
	b, _ := json.Marshal(map[string]any{"partialSuccess": map[string]any{"rejectedSpans": strconv.FormatInt(rejected, 10), "errorMessage": message}})
	return b
}

// StatusJSON is a google.rpc.Status in OTLP/JSON.
func StatusJSON(code int32, message string) []byte {
	b, _ := json.Marshal(map[string]any{"code": code, "message": message})
	return b
}
