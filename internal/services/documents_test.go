package services

import (
	"context"
	"errors"
	"testing"
)

func TestDocumentClassificationValidation(t *testing.T) {
	service := NewDocumentService(nil)
	for _, input := range []CreateDocumentInput{
		{Title: "Document", Stage: "", Type: "OTROS"},
		{Title: "Document", Stage: "OTROS", Type: "  "},
		{Title: "Document", Stage: "Invalid\x00stage", Type: "OTROS"},
	} {
		if _, err := service.Create(context.Background(), 1, input); !errors.Is(err, ErrDocumentInvalid) {
			t.Fatal(err)
		}
	}
}
