package singbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// VerifyRuntimeGeneration observes a JSON generation witness in the engine.
// Callers must separately establish process/controller ownership.
func (c *ClashClient) VerifyRuntimeGeneration(ctx context.Context, tag string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validRuntimeGenerationTag(tag) {
		return fmt.Errorf("invalid expected runtime generation")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.Address()+"/proxies", nil)
	if err != nil {
		return fmt.Errorf("create generation witness request: %w", err)
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request runtime generation witness: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("generation witness HTTP status %d", resp.StatusCode)
	}
	const maxResponseBytes = 4 * 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read runtime generation witness: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return fmt.Errorf("generation witness response too large")
	}
	value, err := decodeGenerationJSON(raw)
	if err != nil {
		return fmt.Errorf("invalid generation witness response")
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid generation witness response object")
	}
	proxies, ok := doc["proxies"].(map[string]any)
	if !ok {
		return fmt.Errorf("generation witness proxies absent")
	}
	for name := range proxies {
		if strings.HasPrefix(name, runtimeGenerationPrefix) && name != tag {
			return fmt.Errorf("competing runtime generation witness")
		}
	}
	proxy, ok := proxies[tag].(map[string]any)
	if !ok {
		return fmt.Errorf("runtime generation absent")
	}
	name, nameOK := proxy["name"].(string)
	kind, kindOK := proxy["type"].(string)
	if !nameOK || name != tag || !kindOK || !strings.EqualFold(kind, "direct") {
		return fmt.Errorf("runtime generation witness mismatch")
	}
	return nil
}
