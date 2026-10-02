package vlink

import (
	"encoding/json"
	"strings"
	"testing"
)

const mieruSimpleSample = "mierus://baozi:manlianpenfen@1.2.3.4?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_HIGH&port=6666&port=9998-9999&port=6489&port=4896&profile=default&protocol=TCP&protocol=TCP&protocol=UDP&protocol=UDP&traffic-pattern=CCoQARoECAEQCiIYCAMQASoIMDAwMTAyMDMqCDA0MDUwNjA3"

const mieruStandardSample = "mieru://CpsBCgdkZWZhdWx0ElgKBWJhb3ppEg1tYW5saWFucGVuZmVuGkA0MGFiYWM0MGY1OWRhNTVkYWQ2YTk5ODMxYTUxMTY1MjJmYmM4MGUzODViYjFhYjE0ZGM1MmRiMzY4ZjczOGE0Gi8SCWxvY2FsaG9zdBoFCIo0EAIaDRACGgk5OTk5LTk5OTkaBQjZMhABGgUIoCYQASD4CioCCAQSB2RlZmF1bHQYnUYguAgwBTgA"

func TestParseMieruSimple_GroupsByTransport(t *testing.T) {
	res := ParseBatch([]string{mieruSimpleSample})
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if len(res.Outbounds) != 2 {
		t.Fatalf("got %d outbounds, want 2", len(res.Outbounds))
	}

	tcp := decodeOutbound(t, res.Outbounds[0])
	if tcp["type"] != "mieru" || tcp["transport"] != "TCP" {
		t.Fatalf("unexpected tcp outbound: %+v", tcp)
	}
	if tcp["server"] != "1.2.3.4" || tcp["username"] != "baozi" || tcp["password"] != "manlianpenfen" {
		t.Fatalf("bad identity fields: %+v", tcp)
	}
	if tcp["server_port"] != float64(6666) {
		t.Fatalf("server_port=%v want 6666", tcp["server_port"])
	}
	assertStringSlice(t, tcp["server_ports"], []string{"9998-9999"})
	if tcp["multiplexing"] != "MULTIPLEXING_HIGH" {
		t.Fatalf("multiplexing=%v", tcp["multiplexing"])
	}
	if tcp["traffic_pattern"] != "CCoQARoECAEQCiIYCAMQASoIMDAwMTAyMDMqCDA0MDUwNjA3" {
		t.Fatalf("traffic_pattern not preserved: %v", tcp["traffic_pattern"])
	}

	udp := decodeOutbound(t, res.Outbounds[1])
	if udp["transport"] != "UDP" || udp["server_port"] != float64(6489) {
		t.Fatalf("unexpected udp outbound: %+v", udp)
	}
	assertStringSlice(t, udp["server_ports"], []string{"4896"})
}

func TestParseMieruStandard_UsesActiveProfile(t *testing.T) {
	res := ParseBatch([]string{mieruStandardSample})
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if len(res.Outbounds) != 2 {
		t.Fatalf("got %d outbounds, want 2", len(res.Outbounds))
	}
	tcp := decodeOutbound(t, res.Outbounds[0])
	if tcp["server"] != "localhost" || tcp["transport"] != "TCP" || tcp["username"] != "baozi" {
		t.Fatalf("bad tcp outbound: %+v", tcp)
	}
	if tcp["server_port"] != float64(6666) {
		t.Fatalf("server_port=%v want 6666", tcp["server_port"])
	}
	assertStringSlice(t, tcp["server_ports"], []string{"9999-9999"})
}

func TestParseMieruSimple_SingleProtocolForAllPorts(t *testing.T) {
	// issue #516: панели кладут один protocol на несколько port'ов
	res := ParseBatch([]string{"mierus://admin:pass@80.100.80.100?multiplexing=MULTIPLEXING_LOW&port=2012&port=2012-2022&profile=mieru&protocol=TCP&traffic-pattern=EAEaAggAIgQIARAB"})
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if len(res.Outbounds) != 1 {
		t.Fatalf("got %d outbounds, want 1", len(res.Outbounds))
	}
	out := decodeOutbound(t, res.Outbounds[0])
	if out["transport"] != "TCP" || out["server_port"] != float64(2012) {
		t.Fatalf("unexpected outbound: %+v", out)
	}
	assertStringSlice(t, out["server_ports"], []string{"2012-2022"})
}

func TestParseMieruSimple_RejectsBadPortProtocolPairs(t *testing.T) {
	links := []string{
		"mierus://u:p@h?profile=default&port=443&protocol=TCP&protocol=UDP",
		"mierus://u:p@h?profile=default&port=443&port=500-600",
	}
	for _, link := range links {
		res := ParseBatch([]string{link})
		if len(res.Outbounds) != 0 {
			t.Fatalf("%s: got outbounds: %+v", link, res.Outbounds)
		}
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "mismatched") {
			t.Fatalf("%s: errors=%+v", link, res.Errors)
		}
	}
}

func TestParseMieruSimple_RejectsInvalidRange(t *testing.T) {
	res := ParseBatch([]string{"mierus://u:p@h?profile=default&port=9999-1&protocol=TCP"})
	if len(res.Outbounds) != 0 {
		t.Fatalf("got outbounds: %+v", res.Outbounds)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "invalid port range") {
		t.Fatalf("errors=%+v", res.Errors)
	}
}

func decodeOutbound(t *testing.T, p ParsedOutbound) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(p.Outbound, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertStringSlice(t *testing.T, v any, want []string) {
	t.Helper()
	gotAny, ok := v.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", v)
	}
	if len(gotAny) != len(want) {
		t.Fatalf("len=%d want %d (%+v)", len(gotAny), len(want), gotAny)
	}
	for i := range want {
		if gotAny[i] != want[i] {
			t.Fatalf("item[%d]=%v want %q", i, gotAny[i], want[i])
		}
	}
}
