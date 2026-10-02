package main

import (
	"fmt"
	"regexp"
	"strings"
)

// commitRe matches a conventional-commit subject with a strict ":" separator,
// optional "(scope)" and optional "!" breaking marker. Capture group 1 is the
// type. " - " separators and non-conventional subjects do not match.
var commitRe = regexp.MustCompile(`^(feat|fix|refactor|perf)(\([^)]+\))?!?:`)

// sections maps a commit type to its changelog heading; slice order is the
// emission order of sections.
var sections = []struct {
	typ     string
	heading string
}{
	{"feat", "Добавлено"},
	{"fix", "Исправлено"},
	{"refactor", "Рефакторинг"},
	{"perf", "Производительность"},
}

// Generate turns commit subjects into a single Keep-a-Changelog block parseable
// by internal/updater.ParseChangelog. Only feat/fix/refactor/perf are kept;
// merge commits and everything else are dropped. Returns "" if nothing qualifies.
func Generate(subjects []string, version, date string) string {
	buckets := map[string][]string{}
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "Merge ") {
			continue
		}
		m := commitRe.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		buckets[m[1]] = append(buckets[m[1]], s)
	}

	total := 0
	for _, sec := range sections {
		total += len(buckets[sec.typ])
	}
	if total == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n\n", version, date)
	for _, sec := range sections {
		items := buckets[sec.typ]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n", sec.heading)
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
		b.WriteString("\n")
	}
	return b.String()
}
