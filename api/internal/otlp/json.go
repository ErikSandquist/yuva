package otlp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

type jsonRequest struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []jsonKeyValue `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []jsonSpan `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

type jsonSpan struct {
	TraceState        string          `json:"traceState"`
	Kind              json.RawMessage `json:"kind"`
	StartTimeUnixNano json.RawMessage `json:"startTimeUnixNano"`
	EndTimeUnixNano   json.RawMessage `json:"endTimeUnixNano"`
	Attributes        []jsonKeyValue  `json:"attributes"`
}

type jsonKeyValue struct {
	Key   string `json:"key"`
	Value struct {
		StringValue *string         `json:"stringValue"`
		IntValue    json.RawMessage `json:"intValue"`
		DoubleValue json.RawMessage `json:"doubleValue"`
	} `json:"value"`
}

func (kv jsonKeyValue) value() value {
	v := kv.Value
	switch {
	case v.StringValue != nil:
		return value{str: *v.StringValue, isStr: true}
	case len(v.IntValue) > 0:
		if n, err := strconv.ParseInt(string(unquote(v.IntValue)), 10, 64); err == nil {
			return value{num: n, isNum: true}
		}
	case len(v.DoubleValue) > 0:
		if f, err := strconv.ParseFloat(string(unquote(v.DoubleValue)), 64); err == nil {
			return value{double: f, isDbl: true}
		}
	}
	return value{}
}

// unquote takes the quotes off a 64-bit number, which OTLP/JSON may send as a string.
func unquote(b []byte) []byte {
	return bytes.Trim(bytes.TrimSpace(b), `"`)
}

// DecodeJSON reads the SERVER spans of an OTLP/JSON ExportTraceServiceRequest. Span kinds may be
// numbers or enum names, 64-bit numbers strings or numbers.
func DecodeJSON(b []byte) ([]Span, error) {
	var req jsonRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, errors.New("the body is not an OTLP/JSON ExportTraceServiceRequest")
	}
	var out []Span
	for _, rs := range req.ResourceSpans {
		res := &Resource{}
		for _, kv := range rs.Resource.Attributes {
			resourceAttr(res, kv.Key, kv.value())
		}
		for _, ss := range rs.ScopeSpans {
			for _, js := range ss.Spans {
				if k := string(unquote(js.Kind)); k != "2" && k != "SPAN_KIND_SERVER" {
					continue
				}
				s := newSpan(res)
				s.TraceState = js.TraceState
				var err error
				if s.Start, err = jsonTime(js.StartTimeUnixNano); err != nil {
					return nil, err
				}
				if s.End, err = jsonTime(js.EndTimeUnixNano); err != nil {
					return nil, err
				}
				for _, kv := range js.Attributes {
					if _, ok := spanKeys[kv.Key]; ok {
						spanAttr(&s, kv.Key, kv.value())
					}
				}
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func jsonTime(b json.RawMessage) (uint64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err := strconv.ParseUint(string(unquote(b)), 10, 64)
	if err != nil {
		return 0, errors.New("startTimeUnixNano and endTimeUnixNano must be nanoseconds")
	}
	return n, nil
}
