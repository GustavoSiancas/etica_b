package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDocumentValidation(t *testing.T) {
	service := NewDocumentService(nil)
	badURL := "http://example.com/doc.pdf"
	for _, input := range []CreateDocumentInput{{Name: ""}, {Name: " \t"}, {Name: "bad\x00name"}, {Name: strings.Repeat("a", 256)}, {Name: "Doc", FileURL: &badURL}} {
		if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrDocumentInvalid) {
			t.Fatalf("accepted invalid document: %v", err)
		}
	}
}
func TestSectionValidationAndNullablePatch(t *testing.T) {
	service := NewDocumentService(nil)
	badColor := "red"
	badID := int64(-1)
	for _, input := range []CreateSectionInput{{Name: "", Type: "OTROS"}, {Name: "Section", Type: " "}, {Name: "Section", Type: "OTROS", SortOrder: -1}, {Name: "Section", Type: "OTROS", ParentID: &badID}, {Name: "Section", Type: "OTROS", ColorHex: &badColor}} {
		if _, err := service.CreateSection(context.Background(), 1, input); !errors.Is(err, ErrSectionInvalid) {
			t.Fatalf("accepted invalid section: %v", err)
		}
	}
	var input UpdateSectionInput
	if err := json.Unmarshal([]byte(`{"expediente_section_parent_id":null,"document_date":null,"sort_order":0}`), &input); err != nil {
		t.Fatal(err)
	}
	if !input.ParentID.Set || input.ParentID.Value != nil || !input.DocumentDate.Set || input.DocumentDate.Value != nil || input.SortOrder == nil || *input.SortOrder != 0 {
		t.Fatal("nulls and zero must be preserved")
	}
	var omitted UpdateSectionInput
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ParentID.Set || omitted.DocumentDate.Set {
		t.Fatal("omitted fields must not change stored values")
	}
	if _, err := service.UpdateSection(context.Background(), 1, 1, omitted); !errors.Is(err, ErrSectionInvalid) {
		t.Fatal("empty PATCH accepted")
	}
}
