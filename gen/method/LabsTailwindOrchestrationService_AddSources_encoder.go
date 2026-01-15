package method

import (
	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
)

// GENERATION_BEHAVIOR: append

// EncodeAddSourcesArgs encodes arguments for LabsTailwindOrchestrationService.AddSources
// RPC ID: izAoDd
// Format (simplified): [%sources%, %project_id%, [2], null, null]
func EncodeAddSourcesArgs(req *notebooklmv1alpha1.AddSourceRequest) []interface{} {
	var sources []interface{}

	for _, src := range req.GetSources() {
		if src.GetUrl() != "" {
			// URL Source format
			sources = append(sources, []interface{}{
				src.GetUrl(),
				nil,
				[]interface{}{2},
			})
		} else {
			// Text/Copy-paste source format
			sources = append(sources, []interface{}{
				nil,
				[]interface{}{src.GetTitle(), src.GetContent()},
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
			})
		}
	}

	return []interface{}{
		[]interface{}{sources},
		req.GetProjectId(),
		[]interface{}{2},
		nil,
		nil,
	}
}
