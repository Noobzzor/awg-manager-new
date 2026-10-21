package warp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func sampleProfile() Profile {
	return Profile{Endpoint: "162.159.193.10:2408", IPv4: "172.16.0.2", MTU: 1280, PrivateKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), PublicKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)), Reserved: []int{0, 127, 255}}
}

func TestProfileValidatesCompleteWARPProfile(t *testing.T) {
	for _, mtu := range []int{1280, 1500} {
		p := sampleProfile()
		p.MTU = mtu
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfileRejectsInvalidFieldsWithoutLeakingValues(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Profile)
	}{
		{"missing endpoint", func(p *Profile) { p.Endpoint = "" }},
		{"hostname", func(p *Profile) { p.Endpoint = "provider.example:2408" }},
		{"no port", func(p *Profile) { p.Endpoint = "162.159.193.10" }},
		{"zero port", func(p *Profile) { p.Endpoint = "162.159.193.10:0" }},
		{"port too high", func(p *Profile) { p.Endpoint = "162.159.193.10:65536" }},
		{"IPv6 endpoint", func(p *Profile) { p.Endpoint = "[2001:db8::1]:2408" }},
		{"multicast endpoint", func(p *Profile) { p.Endpoint = "224.0.0.1:2408" }},
		{"unspecified endpoint", func(p *Profile) { p.Endpoint = "0.0.0.0:2408" }},
		{"mapped endpoint", func(p *Profile) { p.Endpoint = "[::ffff:162.159.193.10]:2408" }},
		{"missing IPv4", func(p *Profile) { p.IPv4 = "" }},
		{"IPv6 address", func(p *Profile) { p.IPv4 = "2001:db8::1" }},
		{"IPv4 prefix", func(p *Profile) { p.IPv4 = "172.16.0.2/32" }},
		{"unspecified IPv4", func(p *Profile) { p.IPv4 = "0.0.0.0" }},
		{"multicast IPv4", func(p *Profile) { p.IPv4 = "224.0.0.1" }},
		{"low MTU", func(p *Profile) { p.MTU = 1279 }},
		{"high MTU", func(p *Profile) { p.MTU = 1501 }},
		{"bad private key", func(p *Profile) { p.PrivateKey = "PRIVATE_SENTINEL" }},
		{"short private key", func(p *Profile) { p.PrivateKey = base64.StdEncoding.EncodeToString([]byte{1}) }},
		{"zero private key", func(p *Profile) { p.PrivateKey = base64.StdEncoding.EncodeToString(make([]byte, 32)) }},
		{"bad public key", func(p *Profile) { p.PublicKey = "PUBLIC_SENTINEL" }},
		{"zero public key", func(p *Profile) { p.PublicKey = base64.StdEncoding.EncodeToString(make([]byte, 32)) }},
		{"missing reserved", func(p *Profile) { p.Reserved = nil }},
		{"short reserved", func(p *Profile) { p.Reserved = []int{1, 2} }},
		{"long reserved", func(p *Profile) { p.Reserved = []int{1, 2, 3, 4} }},
		{"negative reserved", func(p *Profile) { p.Reserved = []int{-1, 2, 3} }},
		{"large reserved", func(p *Profile) { p.Reserved = []int{1, 2, 256} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := sampleProfile()
			tt.change(&p)
			err := p.Validate()
			if err == nil {
				t.Fatal("invalid profile accepted")
			}
			for _, v := range []string{p.PrivateKey, p.PublicKey} {
				if v != "" && strings.Contains(err.Error(), v) {
					t.Fatal("credential leaked")
				}
			}
		})
	}
}

func TestDecodeProfileIsStrictAndBounded(t *testing.T) {
	raw, _ := json.Marshal(sampleProfile())
	p, err := DecodeProfile(bytes.NewReader(raw))
	if err != nil || p.PrivateKey != sampleProfile().PrivateKey {
		t.Fatalf("valid decode: %v", err)
	}
	for _, raw := range []string{string(raw) + ` {}`, string(raw) + ` garbage`, strings.Replace(string(raw), `"mtu":1280`, `"mtu":1280,"unknown":"PRIVATE_SENTINEL"`, 1), `{"privateKey":"PRIVATE_SENTINEL"}`, strings.Repeat(" ", 65537), string(raw) + strings.Repeat(" ", 65537)} {
		got, err := DecodeProfile(strings.NewReader(raw))
		if err == nil {
			t.Fatal("invalid input accepted")
		}
		if got.PrivateKey != "" || got.Reserved != nil {
			t.Fatal("partial credential returned")
		}
		if strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
			t.Fatal("input leaked in error")
		}
	}
}

func TestProfileDescriptionContainsNoCredentials(t *testing.T) {
	p := sampleProfile()
	d := p.Description()
	if !d.Configured || d.Tag != "warp" || d.Endpoint != p.Endpoint || d.IPv4 != p.IPv4 || d.MTU != p.MTU {
		t.Fatal("incomplete public projection")
	}
	raw, _ := json.Marshal(d)
	for _, v := range []string{p.PrivateKey, p.PublicKey, "privateKey", "publicKey", "reserved", "ciphertext"} {
		if bytes.Contains(raw, []byte(v)) {
			t.Fatal("secret projection")
		}
	}
}

func TestDecodeProfileRejectsDuplicateAndCaseAliasFields(t *testing.T) {
	raw, _ := json.Marshal(sampleProfile())
	for _, name := range []string{"mtu", "MTU", "mTu", `\u006dtu`} {
		input := strings.TrimSuffix(string(raw), "}") + `,"` + name + `":1280}`
		if _, err := DecodeProfile(strings.NewReader(input)); err == nil {
			t.Fatalf("duplicate or alias field accepted: %s", name)
		}
	}
	input := strings.Replace(string(raw), `"endpoint"`, `"Endpoint"`, 1)
	if _, err := DecodeProfile(strings.NewReader(input)); err == nil {
		t.Fatal("case alias accepted without canonical field")
	}
}

func TestDecodeProfileRejectsNullReservedElements(t *testing.T) {
	raw, _ := json.Marshal(sampleProfile())
	for _, reserved := range []string{`[null,127,255]`, `[0,null,255]`, `[0,127,null]`, `[null,null,null]`} {
		input := strings.Replace(string(raw), `[0,127,255]`, reserved, 1)
		got, err := DecodeProfile(strings.NewReader(input))
		if err == nil || got.PrivateKey != "" {
			t.Fatalf("null reserved element accepted: %s", reserved)
		}
	}
}
