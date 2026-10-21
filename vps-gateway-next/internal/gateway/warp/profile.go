// Package warp owns imported WARP provider profiles. A valid profile is not
// evidence of successful connection to the Cloudflare service.
package warp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"reflect"
	"strings"
)

const MaxProfileBytes = 64 * 1024
const Tag = "warp"

// Profile contains credentials and must never be returned by public APIs.
type Profile struct {
	Endpoint   string `json:"endpoint"`
	IPv4       string `json:"ipv4"`
	MTU        int    `json:"mtu"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Reserved   []int  `json:"reserved"`
}

type Description struct {
	Configured bool   `json:"configured"`
	Tag        string `json:"tag,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
	IPv4       string `json:"ipv4,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
}

func (p Profile) Validate() error {
	endpoint, err := netip.ParseAddrPort(p.Endpoint)
	if err != nil || endpoint.Port() == 0 || !usableIPv4(endpoint.Addr()) {
		return errors.New("WARP endpoint must be a unicast IPv4 address and nonzero port")
	}
	address, err := netip.ParseAddr(p.IPv4)
	if err != nil || !usableIPv4(address) {
		return errors.New("WARP IPv4 address is invalid")
	}
	if p.MTU < 1280 || p.MTU > 1500 {
		return errors.New("WARP MTU must be between 1280 and 1500")
	}
	if !validKey(p.PrivateKey) {
		return errors.New("WARP private key is invalid")
	}
	if !validKey(p.PublicKey) {
		return errors.New("WARP public key is invalid")
	}
	if len(p.Reserved) != 3 {
		return errors.New("WARP reserved must contain three bytes")
	}
	for _, v := range p.Reserved {
		if v < 0 || v > 255 {
			return errors.New("WARP reserved must contain three bytes")
		}
	}
	return nil
}

func usableIPv4(a netip.Addr) bool { return a.Is4() && a.IsGlobalUnicast() }
func validKey(value string) bool {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 || base64.StdEncoding.EncodeToString(raw) != value {
		return false
	}
	for _, v := range raw {
		if v != 0 {
			return true
		}
	}
	return false
}

func DecodeProfile(reader io.Reader) (Profile, error) {
	raw, err := readBounded(reader)
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := decodeStrict(raw, &p); err != nil {
		return Profile{}, errors.New("invalid WARP profile JSON")
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (p Profile) Description() Description {
	return Description{Configured: true, Tag: Tag, Endpoint: p.Endpoint, IPv4: p.IPv4, MTU: p.MTU}
}

func readBounded(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, MaxProfileBytes+1))
	if err != nil {
		return nil, errors.New("cannot read WARP profile")
	}
	if len(raw) > MaxProfileBytes {
		return nil, errors.New("WARP profile exceeds size limit")
	}
	return raw, nil
}

// Both stored envelopes and profiles are flat objects. Reject duplicate field
// names before struct decoding, including escaped aliases of the same name.
func decodeStrict(raw []byte, out any) error {
	fields := reflect.TypeOf(out).Elem()
	known := make(map[string]bool, fields.NumField())
	for i := 0; i < fields.NumField(); i++ {
		name := strings.Split(fields.Field(i).Tag.Get("json"), ",")[0]
		known[name] = true
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("invalid object")
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || !known[key] || seen[key] {
			return errors.New("duplicate or invalid field")
		}
		seen[key] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return err
		}
		if key == "reserved" {
			// decoding JSON null into int silently yields zero; inspect the
			// original array elements before struct decoding loses that fact.
			var elements []json.RawMessage
			if err := json.Unmarshal(value, &elements); err != nil {
				return err
			}
			for _, element := range elements {
				if bytes.Equal(bytes.TrimSpace(element), []byte("null")) {
					return errors.New("reserved elements must be integers")
				}
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := d.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
