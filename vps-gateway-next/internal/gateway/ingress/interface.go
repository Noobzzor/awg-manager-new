package ingress

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	systemexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

type InterfaceManager struct {
	ipBinary       string
	awgBinary      string
	name           string
	listenPort     int
	privateKey     string
	privateKeyFile string
	address        string
	run            CommandRunner
}

func NewInterfaceManager(ipBinary, awgBinary, name string, listenPort int, privateKey string, run CommandRunner) *InterfaceManager {
	return &InterfaceManager{ipBinary: ipBinary, awgBinary: awgBinary, name: name, listenPort: listenPort, privateKey: privateKey, run: run}
}

func NewLinuxInterfaceManager(name string, listenPort int, privateKey string) *InterfaceManager {
	return NewLinuxInterfaceManagerWithBinary("/opt/sbin/awg", name, listenPort, privateKey)
}

func NewLinuxInterfaceManagerWithBinary(awgBinary, name string, listenPort int, privateKey string) *InterfaceManager {
	manager := NewInterfaceManager("/sbin/ip", awgBinary, name, listenPort, privateKey, func(ctx context.Context, command string, args ...string) (string, error) {
		result, err := systemexec.Run(ctx, command, args...)
		if err != nil {
			if result != nil {
				return result.Stderr, err
			}
			return "", err
		}
		return result.Stdout, nil
	})
	manager.privateKeyFile = "/run/awg-manager/server-private.key"
	return manager
}

func (m *InterfaceManager) SetAddress(address string) {
	if m != nil {
		m.address = address
	}
}

func (m *InterfaceManager) Ensure(ctx context.Context) error {
	if m == nil || m.run == nil || m.ipBinary == "" || m.awgBinary == "" || m.name == "" || m.listenPort < 1 || m.listenPort > 65535 || m.privateKey == "" || m.address == "" {
		return fmt.Errorf("gateway interface manager is not configured")
	}
	privateKeyArg := m.privateKey
	if m.privateKeyFile != "" {
		if err := os.WriteFile(m.privateKeyFile, []byte(m.privateKey+"\n"), 0o600); err != nil {
			return fmt.Errorf("write gateway private key: %w", err)
		}
		privateKeyArg = m.privateKeyFile
	}
	commands := [][]string{
		{m.ipBinary, "link", "add", "dev", m.name, "type", "wireguard"},
		{m.ipBinary, "address", "add", m.address, "dev", m.name},
		{m.awgBinary, "set", m.name, "private-key", privateKeyArg, "listen-port", strconv.Itoa(m.listenPort)},
		{m.ipBinary, "link", "set", "up", "dev", m.name},
	}
	for _, command := range commands {
		output, err := m.run(ctx, command[0], command[1:]...)
		if err != nil {
			lowerOutput := strings.ToLower(output)
			if strings.Contains(lowerOutput, "file exists") && command[1] == "link" && command[2] == "add" {
				continue
			}
			duplicateAddress := command[1] == "address" && command[2] == "add" &&
				(strings.Contains(lowerOutput, "file exists") || strings.Contains(lowerOutput, "address already assigned"))
			if duplicateAddress {
				addresses, inspectErr := m.run(ctx, m.ipBinary, "-o", "address", "show", "dev", m.name)
				if inspectErr != nil {
					return fmt.Errorf("verify gateway interface address: %w", inspectErr)
				}
				if hasInterfaceAddress(addresses, m.address) {
					continue
				}
			}
			if output != "" {
				return fmt.Errorf("configure gateway interface: %s: %w", output, err)
			}
			return fmt.Errorf("configure gateway interface: %w", err)
		}
	}
	return nil
}

func hasInterfaceAddress(output, address string) bool {
	fields := strings.Fields(output)
	for i := 0; i+1 < len(fields); i++ {
		if (fields[i] == "inet" || fields[i] == "inet6") && fields[i+1] == address {
			return true
		}
	}
	return false
}

func (m *InterfaceManager) Delete(ctx context.Context) error {
	if m == nil || m.run == nil || m.ipBinary == "" || m.name == "" {
		return fmt.Errorf("gateway interface manager is not configured")
	}
	if _, err := m.run(ctx, m.ipBinary, "link", "delete", "dev", m.name); err != nil {
		return fmt.Errorf("delete gateway interface: %w", err)
	}
	return nil
}
