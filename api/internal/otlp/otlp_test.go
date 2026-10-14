package otlp

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func otlpFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixturesDecodeAlike(t *testing.T) {
	pb, js := otlpFile(t, "traces.pb"), otlpFile(t, "traces.json")
	fromPB, err := DecodeProto(pb)
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, err := DecodeJSON(js)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromPB) != 9 {
		t.Fatalf("%d SERVER spans in the protobuf export", len(fromPB))
	}
	if !reflect.DeepEqual(fromPB, fromJSON) {
		t.Fatalf("protobuf and JSON differ:\n%+v\n%+v", fromPB, fromJSON)
	}
	want := Resource{Service: "shop-api", Environment: "staging", Version: "1.4.2", Instance: "shop-api-7f9c"}
	first := fromPB[0]
	if *first.Resource != want || first.Method != "GET" || first.Route != "/users/{id}" || first.Path != "/users/42" || first.Status != 200 ||
		first.ClientAddress != "203.0.113.7" || first.Consumer != "mobile-app" || first.APIVersion != "2026-10-01" ||
		first.ResponseSize != 512 || first.RequestSize != -1 || first.End-first.Start != 12_500_000 {
		t.Fatalf("first span %+v", first)
	}
	if grpc := fromPB[7]; grpc.Method != "" || grpc.Status != -1 {
		t.Fatalf("gRPC span %+v", grpc)
	}
}

func withoutRanks(spans []Span) []Span {
	out := make([]Span, len(spans))
	for i, s := range spans {
		s.ranks = [fieldCount]int8{}
		out[i] = s
	}
	return out
}

func TestSemconv120FixtureReadsAsTheCurrentOne(t *testing.T) {
	current, err := DecodeProto(otlpFile(t, "traces.pb"))
	if err != nil {
		t.Fatal(err)
	}
	old, err := DecodeProto(otlpFile(t, "traces-semconv-1.20.pb"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withoutRanks(old), withoutRanks(current)) {
		t.Fatalf("the 1.20 conventions read differently:\n%+v\n%+v", old, current)
	}
	if old[1].ClientAddress != "2001:db8:1::7" || old[2].ClientAddress != "198.51.100.20" || old[3].Path != "/files/0f8fad5b-d9cb-469f-a165-70867728950e" {
		t.Fatalf("old client addresses and paths %+v", old[:4])
	}
}

func TestNewerNamesWinWhateverTheOrder(t *testing.T) {
	body := `{"resourceSpans":[{"resource":{"attributes":[{"key":"deployment.environment","value":{"stringValue":"old"}},
		{"key":"deployment.environment.name","value":{"stringValue":"new"}}]},
		"scopeSpans":[{"spans":[{"kind":2,"startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[
		{"key":"http.method","value":{"stringValue":"POST"}},{"key":"http.request.method","value":{"stringValue":"GET"}},
		{"key":"http.status_code","value":{"intValue":"500"}},{"key":"http.response.status_code","value":{"intValue":"200"}},
		{"key":"net.peer.ip","value":{"stringValue":"10.0.0.1"}},{"key":"client.address","value":{"stringValue":"203.0.113.7"}},
		{"key":"http.url","value":{"stringValue":"https://h/old?x"}},{"key":"http.target","value":{"stringValue":"/target?q=1#f"}},
		{"key":"http.response_content_length","value":{"intValue":"9"}},{"key":"http.response.body.size","value":{"intValue":"7"}},
		{"key":"http.request_content_length","value":{"intValue":"3"}}]}]}]}]}`
	spans, err := DecodeJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	s := spans[0]
	if s.Resource.Environment != "new" || s.Method != "GET" || s.Status != 200 || s.ClientAddress != "203.0.113.7" || s.Path != "/target" ||
		s.ResponseSize != 7 || s.RequestSize != 3 {
		t.Fatalf("span %+v", s)
	}
}

func TestOlderEnvironmentAndKindNames(t *testing.T) {
	body := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"billing"}},
		{"key":"deployment.environment","value":{"stringValue":"qa"}}]},
		"scopeSpans":[{"spans":[{"kind":"SPAN_KIND_SERVER","startTimeUnixNano":1791806401000000000,"endTimeUnixNano":"1791806401000900000",
		"attributes":[{"key":"http.response.status_code","value":{"intValue":"204"}},{"key":"http.request.body.size","value":{"doubleValue":10}}]},
		{"kind":3,"startTimeUnixNano":"1","endTimeUnixNano":"2"}]}]}]}`
	spans, err := DecodeJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].Resource.Environment != "qa" || spans[0].Status != 204 || spans[0].RequestSize != 10 || spans[0].End-spans[0].Start != 900_000 {
		t.Fatalf("spans %+v", spans)
	}
	if _, err := DecodeProto([]byte{0x0a, 0x05, 0x01}); err == nil {
		t.Fatal("a truncated message decoded")
	}
}

func TestAdjustedCount(t *testing.T) {
	for ts, want := range map[string]float64{
		"": 1, "ot=th:0": 1, "ot=th:8": 2, "ot=th:c": 4, "ot=rv:abcdef01234567;th:f": 16, "vendor=x,ot=th:c;rv:fe123456789abc": 4,
		"ot=th:": 1, "ot=th:123456789abcdef": 1, "ot=th:zz": 1, "xot=th:8": 1,
	} {
		if got := AdjustedCount(ts); got != want {
			t.Errorf("%q: %v, want %v", ts, got, want)
		}
	}
}

func TestAnswers(t *testing.T) {
	if len(ResponseProto(0, "")) != 0 || string(ResponseJSON(0, "")) != "{}" {
		t.Fatal("a full success sets partial_success")
	}
	b := ResponseProto(2, "bad")
	num, typ, n := protowire.ConsumeTag(b)
	ps, m := protowire.ConsumeBytes(b[n:])
	if num != 1 || typ != protowire.BytesType || m < 0 || !bytes.Equal(ps, []byte{0x08, 0x02, 0x12, 0x03, 'b', 'a', 'd'}) {
		t.Fatalf("partial success %x", b)
	}
	var doc map[string]map[string]string
	if err := json.Unmarshal(ResponseJSON(2, "bad"), &doc); err != nil || doc["partialSuccess"]["rejectedSpans"] != "2" || doc["partialSuccess"]["errorMessage"] != "bad" {
		t.Fatalf("JSON partial success %v %v", doc, err)
	}
	if !bytes.Equal(StatusProto(CodeUnavailable, "x"), []byte{0x08, 0x0e, 0x12, 0x01, 'x'}) || string(StatusJSON(CodeUnavailable, "x")) != `{"code":14,"message":"x"}` {
		t.Fatal("status")
	}
}
