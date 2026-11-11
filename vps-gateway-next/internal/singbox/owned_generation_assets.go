package singbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	maxOwnedRuleSetBytes = 8 * 1024 * 1024
	maxOwnedRuleSetTotal = 32 * 1024 * 1024
)

func (g *OwnedGenerationProcess) sourceRuleSetName(path string) (string, error) {
	name := filepath.Base(path)
	digest := strings.TrimSuffix(strings.TrimPrefix(name, "sha256-"), ".srs")
	decoded, err := hex.DecodeString(digest)
	if err == nil && len(decoded) == sha256.Size && name == "sha256-"+hex.EncodeToString(decoded)+".srs" {
		for _, store := range []string{"inline", "presets"} {
			relative := filepath.Join("rule-sets", store, name)
			if path == filepath.Join(g.source, relative) {
				return relative, nil
			}
		}
	}
	return "", fmt.Errorf("rule-set path is outside the managed store")
}

// CheckProjectedRouting rejects unsupported inputs before the native checker
// opens the projected candidate. Activation repeats admission on the full merge.
func (g *OwnedGenerationProcess) CheckProjectedRouting(raw []byte) error {
	doc, err := decodeGenerationJSON(raw)
	if err != nil {
		return fmt.Errorf("decode projected rule-set configuration")
	}
	sets, err := g.ownedRuleSetDeclarations(doc)
	if err != nil || len(sets) == 0 {
		return err
	}
	root, err := os.OpenRoot(g.source)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, set := range sets {
		if _, err := g.readSourceRuleSet(root, set["path"].(string)); err != nil {
			return err
		}
	}
	return nil
}

func (g *OwnedGenerationProcess) ownedRuleSetDeclarations(doc any) ([]map[string]any, error) {
	root, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid runtime document")
	}
	route, _ := root["route"].(map[string]any)
	value, present := route["rule_set"]
	// The general asset check may allow matcher references but only these
	// separately validated declarations may carry paths.
	if present {
		delete(route, "rule_set")
		defer func() { route["rule_set"] = value }()
	}
	if !assetFreeGeneration(root) {
		return nil, fmt.Errorf("runtime document references unsupported assets")
	}
	if !present || value == nil {
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok || len(values) > 128 {
		return nil, fmt.Errorf("invalid runtime rule-set list")
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		set, ok := value.(map[string]any)
		path, _ := set["path"].(string)
		tag, _ := set["tag"].(string)
		// Native merge omits the binary format when it is implied by
		// the .srs suffix. Reject other or unrecognized fields.
		fields := 3
		if format, present := set["format"]; present {
			if format != "binary" {
				return nil, fmt.Errorf("unsupported local rule-set format")
			}
			fields++
		}
		if !ok || len(set) != fields || set["type"] != "local" || tag == "" {
			return nil, fmt.Errorf("only compiled managed rule-sets are supported")
		}
		if _, err := g.sourceRuleSetName(path); err != nil {
			return nil, err
		}
		result = append(result, set)
	}
	return result, nil
}

func (g *OwnedGenerationProcess) copyRuleSetAssets(doc any, dir string) (map[string][sha256.Size]byte, error) {
	sets, err := g.ownedRuleSetDeclarations(doc)
	if err != nil || len(sets) == 0 {
		return nil, err
	}
	source, err := os.OpenRoot(g.source)
	if err != nil {
		return nil, fmt.Errorf("open managed rule-set store: %w", err)
	}
	defer source.Close()
	assetsDir := filepath.Join(dir, "assets")
	if err := os.Mkdir(assetsDir, 0700); err != nil {
		return nil, err
	}
	assets := make(map[string][sha256.Size]byte)
	total := 0
	for _, set := range sets {
		raw, err := g.readSourceRuleSet(source, set["path"].(string))
		if err != nil {
			return nil, err
		}
		total += len(raw)
		if total > maxOwnedRuleSetTotal {
			return nil, fmt.Errorf("runtime rule-sets exceed the generation size limit")
		}
		digest := sha256.Sum256(raw)
		relative := filepath.Join("assets", "sha256-"+hex.EncodeToString(digest[:])+".srs")
		if _, exists := assets[relative]; !exists {
			if err := writeOwnedGenerationFile(filepath.Join(dir, relative), raw); err != nil {
				return nil, err
			}
			assets[relative] = digest
		}
		set["path"] = filepath.Join(dir, relative)
	}
	return assets, syncOwnedGenerationDir(assetsDir)
}

func (g *OwnedGenerationProcess) readSourceRuleSet(root *os.Root, path string) ([]byte, error) {
	name, err := g.sourceRuleSetName(path)
	if err != nil {
		return nil, err
	}
	preset := filepath.Dir(name) == filepath.Join("rule-sets", "presets")
	raw, err := readOwnedRuleSet(root, name, preset)
	if err != nil {
		return nil, err
	}
	if preset && filepath.Base(name) != fmt.Sprintf("sha256-%x.srs", sha256.Sum256(raw)) {
		return nil, fmt.Errorf("preset rule-set content does not match its pinned hash")
	}
	return raw, nil
}

func readOwnedRuleSet(root *os.Root, name string, readOnly bool) ([]byte, error) {
	file, err := root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open compiled rule-set: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	var stat unix.Stat_t
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 ||
		(readOnly && info.Mode().Perm() != 0400) || unix.Fstat(int(file.Fd()), &stat) != nil || stat.Nlink != 1 {
		return nil, fmt.Errorf("compiled rule-set has unsafe file attributes")
	}
	if info.Size() < 1 || info.Size() > maxOwnedRuleSetBytes {
		return nil, fmt.Errorf("compiled rule-set has invalid size")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxOwnedRuleSetBytes+1))
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, fmt.Errorf("compiled rule-set changed while being read")
	}
	return raw, nil
}

func (g ownedPreparedGeneration) verifyAssets() error {
	if len(g.assets) == 0 {
		return nil
	}
	root, err := os.OpenRoot(filepath.Dir(g.path))
	if err != nil {
		return err
	}
	defer root.Close()
	for path, digest := range g.assets {
		raw, err := readOwnedRuleSet(root, path, true)
		if err != nil || sha256.Sum256(raw) != digest {
			return fmt.Errorf("prepared rule-set identity changed")
		}
	}
	return nil
}

func writeOwnedGenerationFile(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

func syncOwnedGenerationDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
