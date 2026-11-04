package services

import (
	"context"
	"errors"
	"testing"
)

func TestExpedienteStateAndLocationValidation(t *testing.T) {
	service := NewDocumentService(nil)
	for _, status := range []string{"OPEN", "CLOSED", "IN_PROGRESS", "OTHER", ""} {
		if _, err := service.UpdateFolderStatus(context.Background(), 1, 1, UpdateFolderStatusInput{Status: status}); !errors.Is(err, ErrFolderInvalid) {
			t.Fatalf("invalid status accepted: %q %v", status, err)
		}
	}
	for _, location := range []string{"OTHER", "", "DIRECCION_DE_ETICA"} {
		if _, err := service.UpdateFolderLocation(context.Background(), 1, 1, UpdateFolderLocationInput{Location: location}); !errors.Is(err, ErrFolderInvalid) {
			t.Fatalf("invalid location accepted: %q %v", location, err)
		}
	}
	statuses, _ := service.ListStatuses(context.Background())
	locations, _ := service.ListLocations(context.Background())
	if len(statuses) != 2 || statuses[0].Value != FolderAbierto || statuses[1].Value != FolderCerrado || len(locations) != 3 {
		t.Fatal("incorrect catalogs")
	}
}

func TestExpedienteOpeningRangeValidation(t *testing.T) {
	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"2026-01-01", "", true}, {"", "2026-12-31", true}, {"2026-10-30", "2026-10-30", true},
		{"2026-12-31", "2026-01-01", false}, {"2026-02-30", "", false}, {"", "2026-13-01", false}, {"30/10/2026", "", false}, {"2026-10-30T00:00:00Z", "", false}, {"0000-01-01", "", false},
	} {
		if got := ValidExpedienteFilters(ExpedienteFilters{From: test.from, To: test.to}); got != test.valid {
			t.Fatalf("date range %q %q: %v", test.from, test.to, got)
		}
	}
}
