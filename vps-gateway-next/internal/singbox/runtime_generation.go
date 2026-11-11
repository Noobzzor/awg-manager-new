package singbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const runtimeGenerationPrefix = "awgm-generation-"

// RuntimeGeneration describes prepared JSON and its live engine witness.
// It does not attest process identity or referenced external file contents.
type RuntimeGeneration struct {
	Config []byte
	Tag    string
}

// PrepareRuntimeGeneration prepares a witness for the supplied final JSON.
func PrepareRuntimeGeneration(config []byte) (RuntimeGeneration, error) {
	value, err := decodeGenerationJSON(config)
	if err != nil {
		return RuntimeGeneration{}, fmt.Errorf("invalid runtime JSON")
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return RuntimeGeneration{}, fmt.Errorf("runtime JSON must be an object")
	}
	outbounds, ok := doc["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		return RuntimeGeneration{}, fmt.Errorf("generation witness requires an existing default outbound")
	}
	if hasReservedGenerationReference(doc) {
		return RuntimeGeneration{}, fmt.Errorf("reserved runtime generation namespace")
	}
	canonical, err := json.Marshal(doc)
	if err != nil {
		return RuntimeGeneration{}, fmt.Errorf("encode runtime generation: %w", err)
	}
	digest := sha256.Sum256(canonical)
	tag := runtimeGenerationPrefix + hex.EncodeToString(digest[:])
	doc["outbounds"] = append(outbounds, map[string]any{"type": "direct", "tag": tag})
	prepared, err := json.Marshal(doc)
	if err != nil {
		return RuntimeGeneration{}, fmt.Errorf("encode prepared runtime: %w", err)
	}
	return RuntimeGeneration{Config: prepared, Tag: tag}, nil
}

func validRuntimeGenerationTag(tag string) bool {
	if !strings.HasPrefix(tag, runtimeGenerationPrefix) || len(tag) != len(runtimeGenerationPrefix)+64 || strings.ToLower(tag) != tag {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(tag, runtimeGenerationPrefix))
	return err == nil
}

func hasReservedGenerationReference(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			switch key {
			case "tag", "outbound", "outbounds", "default", "final", "detour", "download_detour":
				if s, ok := child.(string); ok && strings.HasPrefix(s, runtimeGenerationPrefix) {
					return true
				}
				if list, ok := child.([]any); ok {
					for _, item := range list {
						if s, ok := item.(string); ok && strings.HasPrefix(s, runtimeGenerationPrefix) {
							return true
						}
					}
				}
			}
			if hasReservedGenerationReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasReservedGenerationReference(child) {
				return true
			}
		}
	}
	return false
}

// Reject duplicate keys instead of hashing one interpretation of an ambiguous
// document. UseNumber avoids changing large numeric settings while preparing.
func decodeGenerationJSON(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := readGenerationJSONValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	return value, nil
}

func readGenerationJSONValue(d *json.Decoder, depth int) (any, error) {
	if depth > 128 {
		return nil, fmt.Errorf("JSON nesting limit exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, container := token.(json.Delim)
	if !container {
		return token, nil
	}
	switch delim {
	case '{':
		object := make(map[string]any)
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("invalid JSON key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate JSON key")
			}
			value, err := readGenerationJSONValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("invalid JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for d.More() {
			value, err := readGenerationJSONValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("invalid JSON array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("invalid JSON delimiter")
	}
}
