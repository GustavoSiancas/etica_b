package services

import (
	"encoding/json"
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
