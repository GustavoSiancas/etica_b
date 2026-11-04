package services

import (
	"encoding/json"
	"testing"
	"time"

	"etica_back/internal/persistence/models"
)

func TestDocumentDateUTC(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`"2026-11-04"`, "2026-11-04T05:00:00Z"},
		{`"2026-11-04T23:30:00-05:00"`, "2026-11-05T04:30:00Z"},
		{`"2026-11-04T00:00:00+09:00"`, "2026-11-03T15:00:00Z"},
		{`"2026-11-04T05:00:00Z"`, "2026-11-04T05:00:00Z"},
		{`"2024-02-29"`, "2024-02-29T05:00:00Z"},
		{`"2026-11-04T00:00:00.123456-05:00"`, "2026-11-04T05:00:00.123456Z"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var date DocumentDate
			if err := json.Unmarshal([]byte(tc.input), &date); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(date)
			if err != nil || string(got) != `"`+tc.want+`"` {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
			row, err := cleanDocument(CreateDocumentInput{Name: "Test", DocumentDate: &date})
			if err != nil || row.DocumentDate == nil || row.DocumentDate.Format(time.RFC3339Nano) != tc.want {
				t.Fatalf("date not normalized: %+v %v", row, err)
			}
		})
	}
	for _, input := range []string{`"2026-02-29"`, `"2026-02-30"`, `"2026-11-04T12:00:00"`, `"04/11/2026"`, `""`, `42`, `{}`} {
		var date DocumentDate
		if err := json.Unmarshal([]byte(input), &date); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestDocumentDateNullablePatch(t *testing.T) {
	var omitted, cleared, set UpdateDocumentInput
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"document_date":null}`), &cleared); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"document_date":"2026-11-04"}`), &set); err != nil {
		t.Fatal(err)
	}
	if omitted.DocumentDate.Set || !cleared.DocumentDate.Set || cleared.DocumentDate.Value != nil || !set.DocumentDate.Set || set.DocumentDate.Value == nil {
		t.Fatal("PATCH must distinguish omitted, null and date")
	}
}

func TestDocumentResponseUTC(t *testing.T) {
	local := time.Date(2026, 11, 4, 23, 30, 0, 0, time.FixedZone("Peru", -5*60*60))
	row := models.Document{DocumentDate: &local, Audit: models.Audit{CreatedAt: local, UpdatedAt: local}}
	response := documentResponse(row)
	for _, value := range []time.Time{*response.DocumentDate, response.CreatedAt, response.UpdatedAt} {
		if value.Location() != time.UTC || value.Format(time.RFC3339) != "2026-11-05T04:30:00Z" {
			t.Fatalf("expected UTC: %v", value)
		}
	}
}
