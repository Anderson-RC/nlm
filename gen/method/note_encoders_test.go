package method

import (
	"testing"

	notebooklmv1alpha1 "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
)

func TestNoteEncoders(t *testing.T) {
	t.Run("CreateNote", func(t *testing.T) {
		req := &notebooklmv1alpha1.CreateNoteRequest{
			ProjectId: "proj123",
			Title:     "Test Title",
			Content:   "Test Content",
			NoteType:  []int32{1},
		}
		args := EncodeCreateNoteArgs(req)

		// Expected: ["proj123", "Test Content", [1], null, "Test Title"]
		if len(args) != 5 {
			t.Fatalf("expected 5 arguments, got %d", len(args))
		}
		if args[0] != "proj123" {
			t.Errorf("expected arg[0] to be proj123, got %v", args[0])
		}
		if args[1] != "Test Content" {
			t.Errorf("expected arg[1] to be Test Content, got %v", args[1])
		}
		noteType, ok := args[2].([]interface{})
		if !ok || len(noteType) != 1 || (noteType[0] != int64(1)) {
			t.Errorf("expected arg[2] to be list containing 1 (int64), got %T %v (element type: %T)", args[2], args[2], noteType[0])
		}
		if args[3] != nil {
			t.Errorf("expected arg[3] to be nil, got %v", args[3])
		}
		if args[4] != "Test Title" {
			t.Errorf("expected arg[4] to be Test Title, got %v", args[4])
		}
	})

	t.Run("MutateNote", func(t *testing.T) {
		req := &notebooklmv1alpha1.MutateNoteRequest{
			ProjectId: "proj123",
			NoteId:    "note123",
			Updates: []*notebooklmv1alpha1.NoteUpdate{
				{
					Title:   "Updated Title",
					Content: "Updated Content",
				},
			},
		}
		args := EncodeMutateNoteArgs(req)

		// Expected: ["proj123", "note123", [[["Updated Content", "Updated Title", [], 0]]]]
		if len(args) != 3 {
			t.Fatalf("expected 3 arguments, got %d", len(args))
		}
		if args[0] != "proj123" {
			t.Errorf("expected arg[0] to be proj123, got %v", args[0])
		}
		if args[1] != "note123" {
			t.Errorf("expected arg[1] to be note123, got %v", args[1])
		}

		updates, ok := args[2].([]interface{})
		if !ok || len(updates) != 1 {
			t.Fatalf("expected arg[2] to be array of 1 update, got %v", args[2])
		}
		update, ok := updates[0].([]interface{})
		if !ok || len(update) != 1 {
			t.Fatalf("expected update array to have 1 element, got %v", updates[0])
		}
		detail, ok := update[0].([]interface{})
		if !ok || len(detail) != 4 {
			t.Fatalf("expected update detail to have 4 elements, got %v", update[0])
		}
		if detail[0] != "Updated Content" || detail[1] != "Updated Title" {
			t.Errorf("expected detail to be [Updated Content, Updated Title, ...], got %v", detail)
		}
	})

	t.Run("DeleteNotes", func(t *testing.T) {
		req := &notebooklmv1alpha1.DeleteNotesRequest{
			NoteIds: []string{"note1", "note2"},
		}
		args := EncodeDeleteNotesArgs(req)

		// Expected: [null, [["note1", "note2"]], [2, null, [1]]]
		if len(args) != 3 {
			t.Fatalf("expected 3 arguments, got %d", len(args))
		}
		inner := args[1].([]interface{})
		if len(inner) != 1 {
			t.Fatalf("expected inner array to have 1 element, got %d", len(inner))
		}
		ids := inner[0].([]string)
		if len(ids) != 2 || ids[0] != "note1" || ids[1] != "note2" {
			t.Errorf("expected inner[0] to be [note1, note2], got %v", inner[0])
		}
	})
}
