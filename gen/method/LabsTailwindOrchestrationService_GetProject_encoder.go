package method

import (
	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
)

// GENERATION_BEHAVIOR: append

// EncodeGetProjectArgs encodes arguments for LabsTailwindOrchestrationService.GetProject
// RPC ID: rLM1Ne
// Format: [%project_id%, null, [2], null, 0]
func EncodeGetProjectArgs(req *notebooklmv1alpha1.GetProjectRequest) []interface{} {
	return []interface{}{
		req.GetProjectId(),
		nil,
		[]interface{}{2},
		nil,
		0,
	}
}
