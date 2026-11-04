package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"etica_back/internal/services"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeGlobalFolders struct {
	DocumentAPI
	rows          []services.GlobalFolderResponse
	err           error
	limit, offset int
	filters       services.FolderFilters
}

func (f *fakeGlobalFolders) ListAllFolders(_ context.Context, limit, offset int, filters services.FolderFilters) (services.FolderPage, error) {
	f.limit, f.offset, f.filters = limit, offset, filters
	return services.FolderPage{Data: f.rows, Total: int64(len(f.rows)), Limit: limit, Offset: offset}, f.err
}
func TestGlobalFoldersHTTP(t *testing.T) {
	abbreviation := "EP"
	rows := []services.GlobalFolderResponse{{FolderResponse: services.FolderResponse{ID: 7, ExpedienteID: 3, Name: "EXPEDIENTE PRINCIPAL", Abbreviation: &abbreviation, Status: "CERRADO", Location: "CONSEJO DE ETICA"}, ExpedienteNumber: "1-2026", Denomination: "1-2026 EP"}}
	for _, tc := range []struct {
		rows   []services.GlobalFolderResponse
		err    error
		status int
	}{
		{rows, nil, 200}, {[]services.GlobalFolderResponse{}, nil, 200}, {nil, errors.New("private database details"), 500},
	} {
		handler := Routes(nil, &fakeExpedientes{}, &fakeGlobalFolders{rows: tc.rows, err: tc.err}, &fakeParticipants{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/folders", nil))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "private database details") {
			t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
		}
		if tc.status == 200 {
			var page services.FolderPage
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Data == nil || len(page.Data) != len(tc.rows) || page.Limit != 20 || page.Offset != 0 || page.Total != int64(len(tc.rows)) {
				t.Fatalf("invalid global list: %s", w.Body.String())
			}
			if len(page.Data) > 0 && (page.Data[0].Denomination != "1-2026 EP" || page.Data[0].Status != "CERRADO" || page.Data[0].Location != "CONSEJO DE ETICA") {
				t.Fatal("missing identity or current state")
			}
		}
	}
}

func TestGlobalFolderPaginationAndFiltersHTTP(t *testing.T) {
	fake := &fakeGlobalFolders{rows: []services.GlobalFolderResponse{}}
	handler := Routes(nil, &fakeExpedientes{}, fake, &fakeParticipants{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/folders?limit=5&offset=10&status=CERRADO&location=CONSEJO%20DE%20ETICA&folder_type_id=2&expediente_id=3&q=1-2026%20MC&from=2026-11-01&to=2026-11-04", nil))
	if w.Code != 200 || fake.limit != 5 || fake.offset != 10 || fake.filters.Status != "CERRADO" || fake.filters.Location != "CONSEJO DE ETICA" || fake.filters.FolderTypeID == nil || *fake.filters.FolderTypeID != 2 || fake.filters.ExpedienteID == nil || *fake.filters.ExpedienteID != 3 || fake.filters.Q != "1-2026 MC" || fake.filters.From != "2026-11-01" || fake.filters.To != "2026-11-04" {
		t.Fatalf("filters not forwarded: %+v %d %s", fake, w.Code, w.Body.String())
	}
	for _, query := range []string{"limit=0", "limit=101", "offset=-1", "limit=x", "limit=1&limit=2", "status=OPEN", "status=ABIERTO&status=CERRADO", "location=OTHER", "folder_type_id=0", "expediente_id=-1", "expediente_id=x", "folder_type_id=1&folder_type_id=2", "from=2026-02-30", "from=2026-11-05&to=2026-11-04", "to=0000-01-01", "q=a&q=b", "from=2026-11-04&from=2026-11-05"} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/folders?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query %s: %d %s", query, w.Code, w.Body.String())
		}
	}
}
