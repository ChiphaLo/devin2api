package responses

import (
	"fmt"
	"slices"
	"strings"
)

// resolveReasoningModel maps standard effort parameters to real Devin model IDs.
// Unsupported levels and conflicting explicit suffixes fail before inference.
func resolveReasoningModel(model, effort string) (string, error) {
	families := map[string][]string{
		"swe-2":             {"medium", "high", "max"},
		"claude-opus-5-5":   {"low", "medium", "high", "xhigh", "max"},
		"claude-sonnet-5-5": {"low", "medium", "high", "xhigh", "max"},
	}
	for family, levels := range families {
		if model != family && !strings.HasPrefix(model, family+"-") {
			continue
		}
		if model != family {
			if effort == "" || strings.TrimPrefix(model, family+"-") == effort {
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
