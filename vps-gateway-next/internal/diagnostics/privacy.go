package diagnostics

import "strings"

func maskHumanLabel(label string) string {
	r := []rune(label)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 6:
		return strings.Repeat("*", len(r))
	default:
		return string(r[:3]) + strings.Repeat("*", len(r)-6) + string(r[len(r)-3:])
	}
}

func sanitizeReportPrivacy(report *Report) {
	if report == nil {
		return
	}
	if report.WAN.Interfaces != nil {
		for name, iface := range report.WAN.Interfaces {
			iface.Label = maskHumanLabel(iface.Label)
			report.WAN.Interfaces[name] = iface
		}
	}
	report.Privacy = PrivacyInfo{
		Sanitized: true,
		Rules:     []string{"wan-interfaces-labels"},
	}
}
