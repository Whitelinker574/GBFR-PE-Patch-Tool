package main

import "strings"

const (
	LegalityLegal      = "legal"
	LegalityForced     = "forced"
	LegalityUnknown    = "unknown"
	LegalityImpossible = "impossible"
)

// LegalityReport separates game-rule compatibility from binary writability.
// A forced/unknown item is still writable: callers must warn, never silently
// replace the user's requested values.
type LegalityReport struct {
	Status   string   `json:"status"`
	Writable bool     `json:"writable"`
	Message  string   `json:"message"`
	Reasons  []string `json:"reasons"`
}

func newLegalityReport(status string, writable bool, reasons ...string) LegalityReport {
	clean := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if strings.TrimSpace(reason) != "" {
			clean = append(clean, reason)
		}
	}
	message := "符合当前已验证的游戏规则"
	if len(clean) > 0 {
		message = strings.Join(clean, "；")
	}
	return LegalityReport{Status: status, Writable: writable, Message: message, Reasons: clean}
}
