package responses

import (
	"fmt"
	"testing"
)

// TestCPAReasoningModelParameters covers every exposed level and strict rejection.
func TestCPAReasoningModelParameters(t *testing.T) {
	for model, levels := range map[string][]string{
		"swe-2":             {"medium", "high", "max"},
		"swe-2-flash":       {"medium", "high", "max"},
		"claude-opus-5-5":   {"low", "medium", "high", "xhigh", "max"},
		"claude-sonnet-5-5": {"low", "medium", "high", "xhigh", "max"},
	} {
		for _, effort := range append(levels, "") {
			raw := []byte(fmt.Sprintf(`{"model":%q,"reasoning":{"effort":%q},"input":"probe"}`, model, effort))
			got, err := DecodeRequest(raw, true)
			if err != nil {
				t.Fatal(err)
			}
			if effort == "" {
				effort = "medium"
			}
			if got.Context.Model != model+"-"+effort {
				t.Fatalf("variant lost: %s", got.Context.Model)
			}
		}
	}
	for _, raw := range []string{
		`{"model":"swe-2","reasoning":{"effort":"low"},"input":"probe"}`,
		`{"model":"claude-opus-5-5","reasoning":{"effort":"none"},"input":"probe"}`,
		`{"model":"claude-sonnet-5-5","reasoning":{"effort":"ultra"},"input":"probe"}`,
		`{"model":"swe-2-high","reasoning":{"effort":"max"},"input":"probe"}`,
		`{"model":"swe-2-flash","reasoning":{"effort":"none"},"input":"probe"}`,
		`{"model":"swe-2-flash-low","input":"probe"}`,
		`{"model":"swe-2-flash-high","reasoning":{"effort":"max"},"input":"probe"}`,
	} {
		if _, err := DecodeRequest([]byte(raw), true); err == nil {
			t.Fatalf("unsupported or conflicting effort accepted: %s", raw)
		}
	}
}
