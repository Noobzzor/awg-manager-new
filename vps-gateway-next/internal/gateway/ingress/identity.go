package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	systemexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

type ServerIdentity struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

func GenerateKeyPairWithBinary(ctx context.Context, binary string) (string, string, error) {
	if strings.TrimSpace(binary) == "" {
		return "", "", fmt.Errorf("key generator binary is empty")
	}
	privateResult, err := systemexec.Run(ctx, binary, "genkey")
	if err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}
	privateKey := strings.TrimSpace(privateResult.Stdout)
	if privateKey == "" {
		return "", "", fmt.Errorf("generate private key: empty output")
	}
	publicResult, err := systemexec.RunWithOptions(ctx, binary, []string{"pubkey"}, systemexec.Options{Stdin: strings.NewReader(privateKey + "\n")})
	if err != nil {
		return "", "", fmt.Errorf("derive public key: %w", err)
	}
	publicKey := strings.TrimSpace(publicResult.Stdout)
	if publicKey == "" {
		return "", "", fmt.Errorf("derive public key: empty output")
	}
	return privateKey, publicKey, nil
}

func LoadOrCreateServerIdentity(path string, generate func() (string, string, error)) (ServerIdentity, error) {
	if raw, err := os.ReadFile(path); err == nil {
		var identity ServerIdentity
		if err := json.Unmarshal(raw, &identity); err != nil {
			return ServerIdentity{}, fmt.Errorf("decode server identity: %w", err)
		}
		if identity.PrivateKey == "" || identity.PublicKey == "" {
			return ServerIdentity{}, fmt.Errorf("incomplete server identity")
		}
		return identity, nil
	} else if !os.IsNotExist(err) {
		return ServerIdentity{}, fmt.Errorf("read server identity: %w", err)
	}
	if generate == nil {
		return ServerIdentity{}, fmt.Errorf("server identity generator is nil")
	}
	privateKey, publicKey, err := generate()
	if err != nil {
		return ServerIdentity{}, fmt.Errorf("generate server identity: %w", err)
	}
	identity := ServerIdentity{PrivateKey: privateKey, PublicKey: publicKey}
	encoded, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return ServerIdentity{}, fmt.Errorf("encode server identity: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ServerIdentity{}, fmt.Errorf("create server identity directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return ServerIdentity{}, fmt.Errorf("create server identity: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		return ServerIdentity{}, fmt.Errorf("write server identity: %w", err)
	}
	if err := file.Close(); err != nil {
		return ServerIdentity{}, fmt.Errorf("close server identity: %w", err)
	}
	return identity, nil
}
