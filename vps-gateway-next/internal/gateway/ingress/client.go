package ingress

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

type ClientStatus string

const (
	StatusActive   ClientStatus = "active"
	StatusDisabled ClientStatus = "disabled"
	StatusRevoked  ClientStatus = "revoked"
)

var ErrInvalidClient = errors.New("invalid ingress client")

type Client struct {
	ID        string       `json:"id"`
	Label     string       `json:"label"`
	Address   netip.Addr   `json:"address"`
	PublicKey string       `json:"publicKey"`
	Status    ClientStatus `json:"status"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`

	privateKey string
}

type ClientPublic struct {
	ID        string       `json:"id"`
	Label     string       `json:"label"`
	Address   netip.Addr   `json:"address"`
	PublicKey string       `json:"publicKey"`
	Status    ClientStatus `json:"status"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

func NewClient(id, label string, address netip.Addr, privateKey, publicKey string) (*Client, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(label) == "" || !address.IsValid() || !address.Is4() || strings.TrimSpace(privateKey) == "" || strings.TrimSpace(publicKey) == "" {
		return nil, ErrInvalidClient
	}
	now := time.Now().UTC()
	return &Client{ID: id, Label: label, Address: address, PublicKey: publicKey, Status: StatusActive, CreatedAt: now, UpdatedAt: now, privateKey: privateKey}, nil
}

func (c *Client) Public() ClientPublic {
	return ClientPublic{ID: c.ID, Label: c.Label, Address: c.Address, PublicKey: c.PublicKey, Status: c.Status, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func (c ClientPublic) String() string {
	return fmt.Sprintf("client %s (%s) address=%s status=%s publicKey=%s", c.ID, c.Label, c.Address, c.Status, c.PublicKey)
}

func (c *Client) Disable() {
	c.Status = StatusDisabled
	c.UpdatedAt = time.Now().UTC()
}

func (c *Client) Revoke() {
	c.Status = StatusRevoked
	c.UpdatedAt = time.Now().UTC()
}

func (c *Client) Enable() error {
	if c.Status == StatusRevoked {
		return fmt.Errorf("cannot enable revoked client")
	}
	c.Status = StatusActive
	c.UpdatedAt = time.Now().UTC()
	return nil
}
