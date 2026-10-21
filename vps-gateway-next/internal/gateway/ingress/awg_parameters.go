package ingress

import (
	"fmt"
	"strconv"
	"strings"
)

// AWGParameters is one device-level profile shared by server and client export.
// M2 uses scalar H values from the pinned, locally checked userspace backend.
// It has no per-peer AdvancedSecurity switch: that backend rejects the field.
// The legacy WG exporter and registry persistence format are unchanged.
type AWGParameters struct {
	Jc   uint16 `json:"jc"`
	Jmin uint16 `json:"jmin"`
	Jmax uint16 `json:"jmax"`
	S1   uint16 `json:"s1"`
	S2   uint16 `json:"s2"`
	S3   uint16 `json:"s3"`
	S4   uint16 `json:"s4"`
	H1   uint32 `json:"h1"`
	H2   uint32 `json:"h2"`
	H3   uint32 `json:"h3"`
	H4   uint32 `json:"h4"`
}

type awgParameterField struct {
	name  string
	value uint64
}

// Validate admits a bounded M2 operating profile, not every protocol option.
// Limits cap per-handshake junk allocation and padding; they are not a claim
// that this profile fits the eventual packet/VPS budget. Additional options
// remain separate work, not a silent fallback to ordinary WireGuard.
func (p AWGParameters) Validate() error {
	if p.Jc == 0 || p.Jc > 16 {
		return fmt.Errorf("AWG junk count must be between 1 and 16")
	}
	if p.Jmin == 0 || p.Jmax <= p.Jmin || p.Jmax > 1280 {
		return fmt.Errorf("AWG junk sizes must satisfy 0 < Jmin < Jmax <= 1280")
	}
	for _, padding := range []uint16{p.S1, p.S2, p.S3, p.S4} {
		if padding == 0 || padding > 256 {
			return fmt.Errorf("AWG paddings must be between 1 and 256")
		}
	}
	headers := []uint32{p.H1, p.H2, p.H3, p.H4}
	for i, header := range headers {
		if header <= 4 {
			return fmt.Errorf("AWG headers must exclude standard WireGuard message types")
		}
		for _, previous := range headers[:i] {
			if header == previous {
				return fmt.Errorf("AWG headers must not overlap")
			}
		}
	}
	return nil
}

func (p AWGParameters) fields() []awgParameterField {
	return []awgParameterField{
		{"Jc", uint64(p.Jc)}, {"Jmin", uint64(p.Jmin)}, {"Jmax", uint64(p.Jmax)},
		{"S1", uint64(p.S1)}, {"S2", uint64(p.S2)}, {"S3", uint64(p.S3)}, {"S4", uint64(p.S4)},
		{"H1", uint64(p.H1)}, {"H2", uint64(p.H2)}, {"H3", uint64(p.H3)}, {"H4", uint64(p.H4)},
	}
}

// SetArguments contains only public device parameters, never key material.
// The caller supplies the tool, set verb and validated owned interface name.
func (p AWGParameters) SetArguments() ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	args := make([]string, 0, 22)
	for _, field := range p.fields() {
		args = append(args, strings.ToLower(field.name), strconv.FormatUint(field.value, 10))
	}
	return args, nil
}

func RenderAWGClientConfig(client *Client, serverPublicKey, endpoint, clientAddress, presharedKey string, params AWGParameters) (string, error) {
	if err := params.Validate(); err != nil {
		return "", err
	}
	if presharedKey != "" {
		return "", fmt.Errorf("AWG M2 preshared keys require a protected server file contract")
	}
	legacy, err := RenderClientConfig(client, serverPublicKey, endpoint, clientAddress, presharedKey)
	if err != nil {
		return "", err
	}
	prefix, peer, ok := strings.Cut(legacy, "\n\n[Peer]\n")
	if !ok {
		return "", fmt.Errorf("invalid client configuration section boundary")
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('\n')
	for _, field := range params.fields() {
		fmt.Fprintf(&b, "%s = %d\n", field.name, field.value)
	}
	b.WriteString("\n[Peer]\n")
	b.WriteString(peer)
	return b.String(), nil
}

// UseAWGParameters selects the same immutable startup profile for every
// existing private/public export caller. It neither mutates client identities
// nor persists a new registry format. Runtime/profile replacement is not M2.
func (r *Registry) UseAWGParameters(params AWGParameters) error {
	if r == nil {
		return fmt.Errorf("gateway client registry is not configured")
	}
	if err := params.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.awgParameters != nil {
		if *r.awgParameters != params {
			return fmt.Errorf("AWG export profile cannot change while registry is live")
		}
		return nil
	}
	copy := params
	r.awgParameters = &copy
	return nil
}
