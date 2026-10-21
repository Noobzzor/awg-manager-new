package ingress

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

func RenderClientConfig(client *Client, serverPublicKey, endpoint, clientAddress, presharedKey string) (string, error) {
	if client == nil || client.Status == StatusRevoked {
		return "", fmt.Errorf("client cannot export configuration")
	}
	if client.privateKey == "" || serverPublicKey == "" || endpoint == "" {
		return "", fmt.Errorf("incomplete client configuration")
	}
	if strings.ContainsAny(client.privateKey+serverPublicKey+presharedKey, "\r\n") {
		return "", fmt.Errorf("invalid key material")
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid endpoint port")
	}
	prefix, err := netip.ParsePrefix(clientAddress)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is4() {
		return "", fmt.Errorf("invalid client address: %s", clientAddress)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\nAddress = %s\n\n[Peer]\nPublicKey = %s\n", client.privateKey, prefix, serverPublicKey)
	if presharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", presharedKey)
	}
	fmt.Fprintf(&b, "AllowedIPs = 0.0.0.0/0\nEndpoint = %s\nPersistentKeepalive = 25\n", endpoint)
	return b.String(), nil
}

func (r *Registry) RenderClientConfig(id, serverPublicKey, endpoint string) (string, error) {
	client, ok := r.internal(id)
	if !ok {
		return "", ErrClientNotFound
	}
	if client.Status != StatusActive {
		return "", fmt.Errorf("client is not active")
	}
	r.mu.RLock()
	params := r.awgParameters
	dns := r.clientDNS
	r.mu.RUnlock()
	var profile string
	var err error
	if params != nil {
		profile, err = RenderAWGClientConfig(client, serverPublicKey, endpoint, client.Address.String()+"/32", "", *params)
	} else {
		profile, err = RenderClientConfig(client, serverPublicKey, endpoint, client.Address.String()+"/32", "")
	}
	if err != nil {
		return "", err
	}
	if !dns.IsValid() {
		dns = netip.MustParseAddr("1.1.1.1")
	}
	iface, peer, ok := strings.Cut(profile, "\n\n[Peer]\n")
	if !ok {
		return "", fmt.Errorf("invalid client configuration section boundary")
	}
	return iface + "\nDNS = " + dns.String() + "\n\n[Peer]\n" + peer, nil
}
