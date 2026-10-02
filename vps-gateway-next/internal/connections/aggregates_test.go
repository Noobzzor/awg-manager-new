package connections

import (
	"fmt"
	"testing"
)

func TestComputeBuckets_TopNAndOther(t *testing.T) {
	var conns []Connection
	for i := 0; i < 12; i++ {
		for j := 0; j <= i; j++ { // dst-0 → 1 conn, dst-11 → 12 conn
			conns = append(conns, Connection{Dst: fmt.Sprintf("10.0.0.%d", i), BytesIn: 10, BytesOut: 5})
		}
	}
	got := computeBuckets(conns, dstBucketKey)
	if len(got) != 11 { // top-10 + @other
		t.Fatalf("buckets = %d, want 11", len(got))
	}
	if got[0].Key != "10.0.0.11" || got[0].Count != 12 {
		t.Errorf("top bucket = %s/%d, want 10.0.0.11/12", got[0].Key, got[0].Count)
	}
	last := got[10]
	if last.Key != "@other" || last.Count != 1+2 { // dst-0 (1) + dst-1 (2)
		t.Errorf("@other = %s/%d, want @other/3", last.Key, last.Count)
	}
}

func TestBucketKeys(t *testing.T) {
	k, l := tunnelBucketKey(Connection{RouteClass: "singbox"})
	if k != "@singbox" || l != "sing-box" {
		t.Errorf("singbox key/label = %q/%q", k, l)
	}
	k, _ = tunnelBucketKey(Connection{RouteClass: "tunnel", TunnelID: "t1", TunnelName: "VPS"})
	if k != "t1" {
		t.Errorf("tunnel key = %q, want t1", k)
	}
	k, l = clientBucketKey(Connection{ClientName: "Phone", Src: "192.168.0.5"})
	if k != "Phone" || l != "Phone" {
		t.Errorf("client key/label = %q/%q", k, l)
	}
	k, _ = clientBucketKey(Connection{Src: "192.168.0.5"})
	if k != "192.168.0.5" {
		t.Errorf("client fallback key = %q", k)
	}
	k, l = dstBucketKey(Connection{Dst: "1.2.3.4", Rules: []RuleHit{{FQDN: "x.com"}}})
	if k != "1.2.3.4" || l != "x.com" {
		t.Errorf("dst key/label = %q/%q", k, l)
	}
}
