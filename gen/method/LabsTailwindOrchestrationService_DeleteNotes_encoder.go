package method

import (
	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
	"github.com/tmc/nlm/internal/rpc/argbuilder"
)

// GENERATION_BEHAVIOR: append

// EncodeDeleteNotesArgs encodes arguments for LabsTailwindOrchestrationService.DeleteNotes
// RPC ID: AH0mwd
// Updated format based on modern NotebookLM API (January 2026)
// Format: [null, [%note_ids%], [2, null, [1]]]
func EncodeDeleteNotesArgs(req *notebooklmv1alpha1.DeleteNotesRequest) []interface{} {
	// Using generalized argument encoder
	args, err := argbuilder.EncodeRPCArgs(req, "[null, [[%note_ids%]], [2, null, [1]]]")
	if err != nil {
		// Log error and return empty args as fallback
		// In production, this should be handled better
		return []interface{}{}
	}
	return args
}
