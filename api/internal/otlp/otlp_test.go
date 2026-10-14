package otlp

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestFixturesDecodeAlike(t *testing.T) {
	pb, err := os.ReadFile("testdata/traces.pb")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile("testdata/traces.json")
	if err != nil {
		t.Fatal(err)
	}
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
