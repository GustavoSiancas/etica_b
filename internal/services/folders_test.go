package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFolderOpeningDateInput(t *testing.T) {
	for _, value := range []string{"2026-11-04", "2026-11-04T00:00:00-05:00"} {
		var input CreateFolderInput
		if err := json.Unmarshal([]byte(`{"folder_type_id":2,"opening_date":"`+value+`"}`), &input); err != nil {
			t.Fatal(err)
		}
		if input.OpeningDate == nil || input.OpeningDate.Format(time.RFC3339) != "2026-11-04T05:00:00Z" {
			t.Fatalf("unexpected opening date: %+v", input)
		}
	}
	var invalid CreateFolderInput
	if err := json.Unmarshal([]byte(`{"folder_type_id":2,"opening_date":"2026-11-04T00:00:00"}`), &invalid); err == nil {
		t.Fatal("accepted time without timezone")
	}
}

func TestCreateFolderTypeValidation(t *testing.T) {
	service := NewDocumentService(nil)
	for _, name := range []string{"", "  ", "bad\x00name", strings.Repeat("a", 256), string([]byte{0xff})} {
		if _, err := service.CreateFolderType(context.Background(), CreateFolderTypeInput{Name: name}); !errors.Is(err, ErrFolderTypeInvalid) {
			t.Fatalf("invalid name accepted: %q %v", name, err)
		}
	}
}

func TestFolderAbbreviationValidation(t *testing.T) {
	for _, value := range []string{"", "  ", "A B", "A-B", "A\x00B", strings.Repeat("a", 21), string([]byte{0xff})} {
		if _, err := NewDocumentService(nil).CreateFolderType(context.Background(), CreateFolderTypeInput{Name: "Valid name", Abbreviation: value}); !errors.Is(err, ErrFolderTypeInvalid) {
			t.Fatalf("invalid abbreviation accepted: %q %v", value, err)
		}
	}
	for _, tc := range []struct{ input, want string }{{" ep ", "EP"}, {"mc", "MC"}, {"Ee2", "EE2"}} {
		got, err := normalizeFolderAbbreviation(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("normalization %q: %q %v", tc.input, got, err)
		}
	}
}
