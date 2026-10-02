package policy

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"sync"
	"time"
)

// UpstreamKind identifies a gateway egress provider.
type UpstreamKind string

const (
	UpstreamVPN  UpstreamKind = "vpn"
	UpstreamWARP UpstreamKind = "warp"
)

// SelectorMode controls how an upstream group chooses a healthy member.
type SelectorMode string

const (
	SelectorModeExplicit SelectorMode = "selector"
	SelectorModeURLTest  SelectorMode = "urltest"
)

// WARPConfig is the non-secret provider configuration. Credentials are stored
// only as ciphertext produced by EncryptCredentials.
type WARPConfig struct {
	Tag              string `json:"tag"`
	Endpoint         string `json:"endpoint"`
	MTU              int    `json:"mtu"`
	IPv4             string `json:"ipv4,omitempty"`
	IPv6             string `json:"ipv6,omitempty"`
	EncryptedSecrets []byte `json:"encryptedSecrets,omitempty"`
}

func (c WARPConfig) Validate() error {
	if c.Tag == "" {
		return fmt.Errorf("WARP tag is required")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("WARP endpoint is required")
	}
	if c.MTU < 1280 || c.MTU > 1500 {
		return fmt.Errorf("WARP MTU must be between 1280 and 1500")
	}
	if c.IPv4 != "" {
		if addr, err := netip.ParseAddr(c.IPv4); err != nil || !addr.Is4() {
			return fmt.Errorf("invalid WARP IPv4 %q", c.IPv4)
		}
	}
	if c.IPv6 != "" {
		if addr, err := netip.ParseAddr(c.IPv6); err != nil || !addr.Is6() {
			return fmt.Errorf("invalid WARP IPv6 %q", c.IPv6)
		}
	}
	return nil
}

// EncryptCredentials protects WARP credentials before persistence. key must be
// a 32-byte application key supplied by the existing secret store.
func EncryptCredentials(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt WARP credentials: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func DecryptCredentials(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decrypt WARP credentials: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted WARP credentials")
	}
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
}

// Upstream is the health/selection projection used by policy compilation.
type Upstream struct {
	Tag   string
	Kind  UpstreamKind
	Score time.Duration
	Warp  *WARPConfig
}

type HealthState string

const (
	HealthUnknown HealthState = "unknown"
	HealthHealthy HealthState = "healthy"
	HealthFailed  HealthState = "failed"
)

type Health struct {
	State     HealthState `json:"state"`
	CheckedAt time.Time   `json:"checkedAt"`
	Error     string      `json:"error,omitempty"`
}

type Probe func(context.Context, Upstream) error

// HealthRegistry stores current provider health and never turns a failed
// provider into DIRECT. Callers must explicitly choose a different upstream.
type HealthRegistry struct {
	mu     sync.RWMutex
	states map[string]Health
	clock  func() time.Time
}

func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{states: make(map[string]Health), clock: time.Now}
}

func (r *HealthRegistry) Check(ctx context.Context, upstream Upstream, probe Probe) Health {
	err := probe(ctx, upstream)
	h := Health{State: HealthHealthy, CheckedAt: r.clock()}
	if err != nil {
		h.State, h.Error = HealthFailed, err.Error()
	}
	r.mu.Lock()
	r.states[upstream.Tag] = h
	r.mu.Unlock()
	return h
}

func (r *HealthRegistry) Get(tag string) Health {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.states[tag]
}

// SelectUpstream selects only healthy members. URL-test uses the lowest score;
// explicit mode uses the requested tag. No mode silently falls back to DIRECT.
func (r *HealthRegistry) SelectUpstream(mode SelectorMode, requested string, members []Upstream) (Upstream, error) {
	candidates := make([]Upstream, 0, len(members))
	for _, member := range members {
		if r.Get(member.Tag).State == HealthHealthy {
			candidates = append(candidates, member)
		}
	}
	if mode == SelectorModeExplicit {
		for _, member := range candidates {
			if member.Tag == requested {
				return member, nil
			}
		}
		return Upstream{}, fmt.Errorf("requested upstream %q is not healthy", requested)
	}
	if len(candidates) == 0 {
		return Upstream{}, fmt.Errorf("no healthy upstream available")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Tag < candidates[j].Tag
		}
		return candidates[i].Score < candidates[j].Score
	})
	return candidates[0], nil
}
