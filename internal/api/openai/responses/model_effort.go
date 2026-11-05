package responses

import (
	"fmt"
	"slices"
	"strings"
)

// resolveReasoningModel maps standard effort parameters to real Devin model IDs.
// Unsupported levels and conflicting explicit suffixes fail before inference.
func resolveReasoningModel(model, effort string) (string, error) {
	families := []struct {
		name   string
		levels []string
	}{
		{"swe-2-flash", []string{"medium", "high", "max"}},
		{"swe-2", []string{"medium", "high", "max"}},
		{"claude-opus-5-5", []string{"low", "medium", "high", "xhigh", "max"}},
		{"claude-sonnet-5-5", []string{"low", "medium", "high", "xhigh", "max"}},
	}
	// Match the most specific family first; SWE-2 also prefixes SWE-2 Flash.
	for _, entry := range families {
		family, levels := entry.name, entry.levels
		if model != family && !strings.HasPrefix(model, family+"-") {
			continue
		}
		if model != family {
			variant := strings.TrimPrefix(model, family+"-")
			if !slices.Contains(levels, variant) {
				return "", fmt.Errorf("model %q supports reasoning.effort %v", family, levels)
			}
			if effort == "" || variant == effort {
				return model, nil
			}
			return "", fmt.Errorf("reasoning.effort conflicts with explicit model variant %q", model)
		}
		if effort == "" {
			effort = "medium"
		}
		if !slices.Contains(levels, effort) {
			return "", fmt.Errorf("model %q supports reasoning.effort %v, not %q", model, levels, effort)
		}
		return family + "-" + effort, nil
	}
	if effort != "" {
		return "", fmt.Errorf("reasoning.effort mapping is not configured for model %q", model)
	}
	return model, nil
}
