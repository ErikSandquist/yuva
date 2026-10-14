package otlp

import (
	"errors"
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

var errMalformed = errors.New("the body is not an ExportTraceServiceRequest")

type field struct {
	num protowire.Number
	typ protowire.Type
	b   []byte
	x   uint64
}

// fields calls f for each field of a message; groups and unknown fields are skipped.
func fields(b []byte, f func(field) error) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return errMalformed
		}
		b = b[n:]
		fd := field{num: num, typ: typ}
		switch typ {
		case protowire.BytesType:
			fd.b, n = protowire.ConsumeBytes(b)
		case protowire.VarintType:
			fd.x, n = protowire.ConsumeVarint(b)
		case protowire.Fixed64Type:
			fd.x, n = protowire.ConsumeFixed64(b)
		case protowire.Fixed32Type:
			var v uint32
			v, n = protowire.ConsumeFixed32(b)
			fd.x = uint64(v)
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
		}
		if n < 0 {
			return errMalformed
		}
		b = b[n:]
		if err := f(fd); err != nil {
			return err
		}
	}
	return nil
}

func (f field) bytes(num protowire.Number) ([]byte, bool, error) {
	if f.num != num {
		return nil, false, nil
	}
	if f.typ != protowire.BytesType {
		return nil, false, errMalformed
	}
	return f.b, true, nil
}

// DecodeProto reads the SERVER spans of a binary protobuf ExportTraceServiceRequest.
func DecodeProto(b []byte) ([]Span, error) {
	var out []Span
	err := fields(b, func(f field) error {
		rs, ok, err := f.bytes(1)
		if !ok || err != nil {
			return err
		}
		var res []byte
		var scopes [][]byte
		if err := fields(rs, func(f field) error {
			switch f.num {
			case 1:
				r, _, err := f.bytes(1)
				res = r
				return err
			case 2:
				s, _, err := f.bytes(2)
				scopes = append(scopes, s)
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		resource := &Resource{}
		if err := fields(res, func(f field) error {
			kv, ok, err := f.bytes(1)
			if !ok || err != nil {
				return err
			}
			return keyValue(kv, func(string) bool { return true }, func(k string, v value) { resourceAttr(resource, k, v) })
		}); err != nil {
			return err
		}
		for _, scope := range scopes {
			if err := fields(scope, func(f field) error {
				sp, ok, err := f.bytes(2)
				if !ok || err != nil {
					return err
				}
				s, server, err := protoSpan(sp, resource)
				if server {
					out = append(out, s)
				}
				return err
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func protoSpan(b []byte, res *Resource) (Span, bool, error) {
	s := newSpan(res)
	var kind uint64
	var attrs [][]byte
	err := fields(b, func(f field) error {
		switch f.num {
		case 3:
			v, _, err := f.bytes(3)
			s.TraceState = string(v)
			return err
		case 6:
			if f.typ != protowire.VarintType {
				return errMalformed
			}
			kind = f.x
		case 7, 8:
			if f.typ != protowire.Fixed64Type {
				return errMalformed
			}
			if f.num == 7 {
				s.Start = f.x
			} else {
				s.End = f.x
			}
		case 9:
			v, _, err := f.bytes(9)
			attrs = append(attrs, v)
			return err
		}
		return nil
	})
	if err != nil || kind != KindServer {
		return s, false, err
	}
	for _, kv := range attrs {
		if err := keyValue(kv, func(k string) bool { return spanKeys[k] }, func(k string, v value) { spanAttr(&s, k, v) }); err != nil {
			return s, false, err
		}
	}
	return s, true, nil
}

// keyValue reads a KeyValue and hands its value to set when want accepts its key.
func keyValue(b []byte, want func(string) bool, set func(string, value)) error {
	var key string
	var raw []byte
	if err := fields(b, func(f field) error {
		switch f.num {
		case 1:
			v, _, err := f.bytes(1)
			key = string(v)
			return err
		case 2:
			v, _, err := f.bytes(2)
			raw = v
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if !want(key) {
		return nil
	}
	var v value
	if err := fields(raw, func(f field) error {
		switch {
		case f.num == 1 && f.typ == protowire.BytesType:
			v = value{str: string(f.b), isStr: true}
		case f.num == 3 && f.typ == protowire.VarintType:
			v = value{num: int64(f.x), isNum: true}
		case f.num == 4 && f.typ == protowire.Fixed64Type:
			v = value{double: math.Float64frombits(f.x), isDbl: true}
		}
		return nil
	}); err != nil {
		return err
	}
	set(key, v)
	return nil
}

// ResponseProto is a binary ExportTraceServiceResponse; partial_success is set only when spans
// were rejected.
func ResponseProto(rejected int64, message string) []byte {
	if rejected == 0 && message == "" {
		return []byte{}
	}
	var ps []byte
	if rejected != 0 {
		ps = protowire.AppendTag(ps, 1, protowire.VarintType)
		ps = protowire.AppendVarint(ps, uint64(rejected))
	}
	if message != "" {
		ps = protowire.AppendTag(ps, 2, protowire.BytesType)
		ps = protowire.AppendString(ps, message)
	}
	out := protowire.AppendTag(nil, 1, protowire.BytesType)
	return protowire.AppendBytes(out, ps)
}

// StatusProto is a binary google.rpc.Status.
func StatusProto(code int32, message string) []byte {
	out := protowire.AppendTag(nil, 1, protowire.VarintType)
	out = protowire.AppendVarint(out, uint64(int64(code)))
	if message != "" {
		out = protowire.AppendTag(out, 2, protowire.BytesType)
		out = protowire.AppendString(out, message)
	}
	return out
}
