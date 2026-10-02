package policy

import "net/netip"

// Runtime owns the gateway policy apply path. It deliberately depends only on
// the AWG3 catalog projection and the orchestrator slot saver, so server wiring
// can inject existing production services without duplicating compilation rules.
type Runtime struct {
	Catalog          AWG3Catalog
	Saver            SlotSaver
	ClientPool       netip.Prefix
	IngressInterface string
	WANInterface     string
}

// Apply compiles one portable profile against the currently available AWG3
// endpoints and writes the deterministic result through the orchestrator slot.
func (r Runtime) Apply(profile Profile, selectedVPNTag string) error {
	return r.ApplyWithSources(profile, selectedVPNTag, nil, nil)
}

// ApplyWithSources resolves client and group membership to gateway ingress
// addresses before compilation. A missing reference is rejected by Compile,
// before the orchestrator slot is touched.
func (r Runtime) ApplyWithSources(profile Profile, selectedVPNTag string, clients, groups map[string][]string) error {
	compiled, err := r.PreviewWithSources(profile, selectedVPNTag, clients, groups)
	if err != nil {
		return err
	}
	return ApplyToSlot(r.Saver, compiled)
}

// PreviewWithSources compiles a profile against the current AWG3 catalog and
// ingress source bindings without writing the gateway policy slot.
func (r Runtime) PreviewWithSources(profile Profile, selectedVPNTag string, clients, groups map[string][]string) (CompiledPolicy, error) {
	options, err := CompileOptionsFromAWG3(r.Catalog, selectedVPNTag)
	if err != nil {
		return CompiledPolicy{}, err
	}
	options.Clients = clients
	options.Groups = groups
	options.ClientPool = r.ClientPool
	options.IngressInterface = r.IngressInterface
	options.WANInterface = r.WANInterface
	compiled, err := Compile(profile, options)
	if err != nil {
		return CompiledPolicy{}, err
	}
	return compiled, nil
}
