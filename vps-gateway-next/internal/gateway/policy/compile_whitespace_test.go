package policy

import "testing"

func TestCompileNormalizesTransportWhitespace(t *testing.T) {
	for _, protocol := range []string{" tcp ", "\tudp\n", " icmp "} {
		t.Run(protocol, func(t *testing.T) {
			profile := Profile{ID: "p", Name: "P", DefaultAction: ActionBlock,
				Rules: []Rule{{ID: "transport", Enabled: true, Action: ActionBlock, Protocols: []string{protocol}}}}
			compiled, err := Compile(profile, CompileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{" tcp ": "tcp", "\tudp\n": "udp", " icmp ": "icmp"}[protocol]
			if len(compiled.Rules) != 1 || len(compiled.Rules[0].Network) != 1 || compiled.Rules[0].Network[0] != want {
				t.Fatalf("network matcher not canonical: %#v", compiled.Rules)
			}
			if profile.Rules[0].Protocols[0] != protocol {
				t.Fatal("compile mutated input profile")
			}
		})
	}
}
