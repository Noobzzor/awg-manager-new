package router

import (
	"errors"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// GCRuleSetArtifacts removes rule-set artifact files (rule-sets/inline/*.{json,srs},
// rule-sets/dat/*.{json,srs,meta.json}) that no config references anymore
// (issue #448: deletes and renames of rule-sets never cleaned their files).
//
// The referenced set is the UNION of the APPLIED configs (active/, then
// disabled/ — what sing-box may be running) and the PENDING drafts (what the
// user is editing), for BOTH the router and fakeip slots — both slots
// materialize into the same rule-sets/ dirs. A rule-set deleted only in the
// pending draft therefore keeps its files until the draft is applied
// (TestDeleteRuleSet_StagedInlineKeepsSRSCompanionFiles), and a discarded
// draft never loses the active artifacts.
//
// Called after a successful ApplyStaging and once at boot (cmd/awg-manager)
// to reap historical leftovers. Best-effort: any config read error aborts the
// sweep (fail-safe — never GC against an incomplete referenced set).
func (s *ServiceImpl) GCRuleSetArtifacts() {
	referenced, err := s.referencedRuleSetArtifactBases()
	if err != nil {
		s.appLog.Warn("gc", "rule-sets", "skip rule-set artifact GC: "+err.Error())
		return
	}
	s.ruleSetMaterializer().gcArtifacts(referenced)
}

// referencedRuleSetArtifactBases builds the set of artifact base names still
// referenced by any router/fakeip config (applied or pending).
func (s *ServiceImpl) referencedRuleSetArtifactBases() (map[string]struct{}, error) {
	referenced := make(map[string]struct{})

	// Router slot: effective (pending draft if present, else active) + applied.
	cfg, err := s.loadRouterConfig()
	if err != nil {
		return nil, err
	}
	addRuleSetArtifactBases(referenced, cfg)
	cfg, err = s.loadAppliedRouterConfig()
	if err != nil {
		return nil, err
	}
	addRuleSetArtifactBases(referenced, cfg)

	// FakeIP slot (orch-only): it materializes into the same rule-sets/ dirs.
	if s.deps.Orch != nil {
		cfg, err = s.loadFakeIPConfig()
		if err != nil {
			return nil, err
		}
		addRuleSetArtifactBases(referenced, cfg)
		data, err := s.deps.Orch.LoadApplied(orchestrator.SlotFakeIP)
		if err != nil {
			if !errors.Is(err, orchestrator.ErrUnknownSlot) {
				return nil, err
			}
		} else if data != nil {
			applied, err := parseRouterConfigBytes(data)
			if err != nil {
				return nil, err
			}
			addRuleSetArtifactBases(referenced, applied)
		}
	}
	return referenced, nil
}

// addRuleSetArtifactBases records every artifact base name cfg can reference:
//   - inline rule-sets and their materialized SRS companions — the file base
//     is safeRuleSetFilename(<inline tag>) (materializeRuleSet names the files
//     after the inline tag, while the companion RuleSet entry carries the
//     "<tag>-srs" tag); both tag variants are added defensively;
//   - remote rule-sets whose URL points at the local dat-srs endpoint — the
//     base is datRuleSetBaseName(kind, tags), matching DatRuleSetFile.
func addRuleSetArtifactBases(set map[string]struct{}, cfg *RouterConfig) {
	if cfg == nil {
		return
	}
	for _, rs := range cfg.Route.RuleSet {
		if rs.Tag != "" {
			for _, tag := range ruleSetTagsWithCompanion(rs.Tag) {
				set[safeRuleSetFilename(tag)] = struct{}{}
			}
			if base, ok := inlineTagFromSRSTag(rs.Tag); ok {
				set[safeRuleSetFilename(base)] = struct{}{}
			}
		}
		if kind, tags, ok := parseDatRuleSetURL(rs.URL); ok {
			set[datRuleSetBaseName(kind, tags)] = struct{}{}
		}
	}
}
