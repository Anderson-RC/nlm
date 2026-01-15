package method

import (
	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
	"github.com/tmc/nlm/internal/rpc/argbuilder"
)

// GENERATION_BEHAVIOR: append

// EncodeCreateNoteArgs encodes arguments for LabsTailwindOrchestrationService.CreateNote
// RPC ID: CYK0Xb
// Updated format based on research (January 2026)
// Format: [project_id, content, note_type, null, title]
// This maps directly to field numbers 1, 2, 3, (skip 4), 5.
func EncodeCreateNoteArgs(req *notebooklmv1alpha1.CreateNoteRequest) []interface{} {
	// Using generalized argument encoder
	args, err := argbuilder.EncodeRPCArgs(req, "[%project_id%, %content%, %note_type%, null, %title%]")
	if err != nil {
		// Log error and return empty args as fallback
		// In production, this should be handled better
		return []interface{}{}
	}
	return args
}
