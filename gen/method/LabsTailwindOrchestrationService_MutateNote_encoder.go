package method

import (
	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
)

// GENERATION_BEHAVIOR: append

// EncodeMutateNoteArgs encodes arguments for LabsTailwindOrchestrationService.MutateNote
// RPC ID: cYAfTb
// Updated format based on modern NotebookLM API (January 2026)
// Format: [project_id, note_id, [[[content, title, [], 0]]]]
func EncodeMutateNoteArgs(req *notebooklmv1alpha1.MutateNoteRequest) []interface{} {
	title := ""
	content := ""
	if len(req.GetUpdates()) > 0 {
		title = req.GetUpdates()[0].GetTitle()
		content = req.GetUpdates()[0].GetContent()
	}

	return []interface{}{
		req.GetProjectId(),
		req.GetNoteId(),
		[]interface{}{
			[]interface{}{
				[]interface{}{
					content,
					title,
					[]string{},
					0,
				},
			},
		},
	}
}
