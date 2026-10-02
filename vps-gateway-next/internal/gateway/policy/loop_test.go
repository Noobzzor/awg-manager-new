package policy

import (
	"strings"
	"testing"
)

func TestCompileRejectsOutboundThatIsAnInboundTag(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:       "loop",
			Action:   ActionVPN,
			Outbound: "client-in",
			Enabled:  true,
		}},
	}

	_, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "client-in": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "client-in": ActionVPN},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
		InboundTags:      map[string]struct{}{"client-in": {}},
	})
	if err == nil || !strings.Contains(err.Error(), "route loop") {
		t.Fatalf("expected route loop error, got %v", err)
	}
}
