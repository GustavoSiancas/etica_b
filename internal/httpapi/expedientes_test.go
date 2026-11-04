package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"etica_back/internal/services"
)

type fakeExpedientes struct {
	descriptionID    int64
	descriptionInput services.UpdateExpedienteDescriptionInput
	participantID    int64
	participantRole  string
	participantErr   error
	filters          services.ExpedienteFilters
	createErr        error
	limit, offset    int
	statusErr        error
}
type fakeDocuments struct {
	locationInput services.UpdateFolderLocationInput
	locationID    int64
	statusInput   services.UpdateFolderStatusInput
	statusID      int64
	statusErr     error
	DocumentAPI
	input        services.CreateDocumentInput
	create       services.DocumentResponse
	list         []services.DocumentResponse
	err          error
	expedienteID int64
}

type fakeParticipants struct {
	filters       services.ParticipantFilters
	limit, offset int
	list          []services.ParticipantMembershipResponse
	listErr       error
	expedienteID  int64
	kind          string
}

func (f *fakeParticipants) ListDenunciantesByExpediente(_ context.Context, id int64) ([]services.ParticipantMembershipResponse, error) {
	f.expedienteID, f.kind = id, "denunciantes"
	if f.list == nil {
		return []services.ParticipantMembershipResponse{}, f.listErr
	}
	return f.list, f.listErr
}
func (f *fakeParticipants) ListDenunciadosByExpediente(_ context.Context, id int64) ([]services.ParticipantMembershipResponse, error) {
	f.expedienteID, f.kind = id, "denunciados"
	if f.list == nil {
		return []services.ParticipantMembershipResponse{}, f.listErr
	}
	return f.list, f.listErr
}

func (*fakeParticipants) CreateDenunciante(_ context.Context, input services.CreateDenuncianteInput) (services.ParticipantResponse, error) {
	return services.ParticipantResponse{ID: 1, Name: input.Name}, nil
}
func (*fakeParticipants) CreateDenunciado(_ context.Context, input services.CreateDenunciadoInput) (services.ParticipantResponse, error) {
	return services.ParticipantResponse{ID: 2, Name: input.Name}, nil
}
func (f *fakeParticipants) ListDenunciantes(_ context.Context, limit, offset int, filters services.ParticipantFilters) ([]services.ParticipantResponse, int64, error) {
	f.filters, f.limit, f.offset = filters, limit, offset
	return []services.ParticipantResponse{}, 0, nil
}
func (f *fakeParticipants) ListDenunciados(_ context.Context, limit, offset int, filters services.ParticipantFilters) ([]services.ParticipantResponse, int64, error) {
	f.filters, f.limit, f.offset = filters, limit, offset
	return []services.ParticipantResponse{}, 0, nil
}
func (*fakeParticipants) AttachDenunciante(_ context.Context, expedienteID, participantID int64) (services.ParticipantMembershipResponse, error) {
	return services.ParticipantMembershipResponse{ID: 5, ExpedienteID: expedienteID, Participant: services.ParticipantResponse{ID: participantID, Name: "Ana"}}, nil
}
func (*fakeParticipants) AttachDenunciado(_ context.Context, expedienteID, participantID int64) (services.ParticipantMembershipResponse, error) {
	return services.ParticipantMembershipResponse{ID: 6, ExpedienteID: expedienteID, Participant: services.ParticipantResponse{ID: participantID, Name: "Luis"}}, nil
}

func (f *fakeDocuments) Create(_ context.Context, input services.CreateDocumentInput) (services.DocumentResponse, error) {
	f.input = input
	if f.err != nil {
		return services.DocumentResponse{}, f.err
	}
	return f.create, nil
}
func (f *fakeDocuments) ListByExpediente(_ context.Context, id int64) ([]services.DocumentResponse, error) {
	f.expedienteID = id
	return f.list, f.err
}

func (f *fakeExpedientes) Create(_ context.Context, input services.CreateExpedienteInput) (services.ExpedienteResponse, error) {
	initType := input.InitType
	return services.ExpedienteResponse{ID: 1, Number: "1-2026", InitType: &initType}, f.createErr
}
func (f *fakeExpedientes) ListInitTypes(context.Context) ([]services.ExpedienteInitTypeResponse, error) {
	return []services.ExpedienteInitTypeResponse{{Value: "DENUNCIA_DE_OFICIO", Label: "DENUNCIA DE OFICIO"}}, nil
}
func (f *fakeDocuments) ListLocations(ctx context.Context) ([]services.FolderLocationResponse, error) {
	return services.NewDocumentService(nil).ListLocations(ctx)
}
func (f *fakeDocuments) UpdateFolderLocation(_ context.Context, expedienteID, id int64, input services.UpdateFolderLocationInput) (services.FolderResponse, error) {
	f.locationID, f.locationInput = id, input
	return services.FolderResponse{ID: id, ExpedienteID: expedienteID, Location: input.Location}, f.statusErr
}
func (f *fakeDocuments) ListStatuses(context.Context) ([]services.FolderStatusResponse, error) {
	return []services.FolderStatusResponse{{Value: "ABIERTO", Label: "Abierto"}, {Value: "CERRADO", Label: "Cerrado"}}, nil
}
func (f *fakeExpedientes) List(_ context.Context, limit, offset int, filters ...services.ExpedienteFilters) (services.ExpedienteList, error) {
	f.limit = limit
	f.offset = offset
	if len(filters) > 0 {
		f.filters = filters[0]
	}
	return services.ExpedienteList{Data: []services.ExpedienteResponse{}, Limit: limit, Offset: offset}, nil
}
func (f *fakeDocuments) UpdateFolderStatus(_ context.Context, expedienteID, id int64, input services.UpdateFolderStatusInput) (services.FolderResponse, error) {
	f.statusID, f.statusInput = id, input
	return services.FolderResponse{ID: id, ExpedienteID: expedienteID, Status: input.Status}, f.statusErr
}

func TestCreateExpedienteHTTP(t *testing.T) {
	for _, test := range []struct {
		body   string
		err    error
		status int
	}{
		{`{"description":"Caso de prueba","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_OFICIO"}`, nil, 201},
		{`{"description":"Caso de prueba","init_type":"DENUNCIA_DE_OFICIO"}`, services.ErrExpedienteDuplicate, 409},
		{`{"number":""}`, nil, 400},
		{`{"description":"Caso","init_type":"UNKNOWN"}`, services.ErrExpedienteInvalid, 400},
		{`{"number":"EXP-1","status":"CERRADO"}`, nil, 400},
		{`{"description":"Caso","init_type":"DENUNCIA_DE_OFICIO"}`, errors.New("private database information"), 500},
	} {
		handler := Routes(nil, &fakeExpedientes{createErr: test.err}, &fakeDocuments{}, &fakeParticipants{})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/expedientes", strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Fatalf("got %d want %d", response.Code, test.status)
		}
		if strings.Contains(response.Body.String(), "private database information") {
			t.Fatal("exposed database error")
		}
		if test.status == 201 {
			if strings.Contains(response.Body.String(), `"status":`) || strings.Contains(response.Body.String(), `"location":`) {
				t.Fatal("expediente still owns folder fields")
			}
			var data services.ExpedienteResponse
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || data.ID != 1 || data.InitType == nil || *data.InitType != "DENUNCIA_DE_OFICIO" {
				t.Fatal("incorrect create response")
			}
		}
	}
}
func TestListExpedientesHTTP(t *testing.T) {
	fake := &fakeExpedientes{}
	handler := Routes(nil, fake, &fakeDocuments{}, &fakeParticipants{})
	for _, test := range []struct {
		query                 string
		status, limit, offset int
	}{
		{"", 200, 20, 0}, {"?limit=5&offset=10", 200, 5, 10},
		{"?limit=0", 400, 0, 0}, {"?limit=101", 400, 0, 0}, {"?offset=-1", 400, 0, 0}, {"?limit=invalid", 400, 0, 0}, {"?limit=1&limit=2", 400, 0, 0},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/expedientes"+test.query, nil))
		if response.Code != test.status {
			t.Fatalf("%s got %d", test.query, response.Code)
		}
		if test.status == 200 {
			if fake.limit != test.limit || fake.offset != test.offset {
				t.Fatal("incorrect pagination")
			}
			if !strings.Contains(response.Body.String(), `"data":[]`) {
				t.Fatal("empty list must be an array")
			}
		}
	}
}

func TestUpdateFolderStatusHTTP(t *testing.T) {
	fake := &fakeDocuments{}
	handler := Routes(nil, &fakeExpedientes{}, fake, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("PATCH", "/expedientes/42/folders/42/status", strings.NewReader(`{"status":"CERRADO"}`)))
	if response.Code != 200 || fake.statusID != 42 || fake.statusInput.Status != "CERRADO" || !strings.Contains(response.Body.String(), `"status":"CERRADO"`) {
		t.Fatal("expediente status update endpoint failed")
	}
	for _, test := range []struct {
		path, body string
		status     int
	}{
		{"/expedientes/x/folders/42/status", `{"status":"ABIERTO"}`, 400},
		{"/expedientes/0/folders/42/status", `{"status":"ABIERTO"}`, 400},
		{"/expedientes/42/folders/42/status", `{"status":"ABIERTO","extra":true}`, 400},
	} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("PATCH", test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Errorf("%s got %d want %d", test.path, response.Code, test.status)
		}
	}
	fake.statusErr = services.ErrExpedienteNotFound
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("PATCH", "/expedientes/999/folders/42/status", strings.NewReader(`{"status":"CERRADO"}`)))
	if response.Code != 404 {
		t.Fatal("missing expediente status update should return 404")
	}
}

func TestListInitTypes(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/tipos-inicio", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "DENUNCIA DE OFICIO") {
		t.Fatal("init type catalog endpoint failed")
	}
}

func TestListFolderStatuses(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/folder-statuses", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"value":"ABIERTO"`) || !strings.Contains(response.Body.String(), `"value":"CERRADO"`) {
		t.Fatal("expediente status catalog endpoint failed")
	}
}

func TestParticipantEndpoints(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/denunciantes", `{"name":"Ana"}`, 404},
		{"GET", "/denunciantes?limit=10", ``, 200},
		{"POST", "/denunciados", `{"name":"Luis"}`, 404},
		{"GET", "/denunciados", ``, 200},
		{"POST", "/expedientes/4/denunciantes", `{"denunciante_id":1}`, 405},
		{"POST", "/expedientes/4/denunciados", `{"denunciado_id":2}`, 405},
		{"POST", "/expedientes/x/denunciantes", `{"denunciante_id":1}`, 405},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Errorf("%s %s: got %d want %d", test.method, test.path, response.Code, test.status)
		}
	}
}

func TestFixedParticipantsByExpedienteHTTP(t *testing.T) {
	for _, kind := range []string{"denunciantes", "denunciados"} {
		for _, tc := range []struct {
			id     string
			err    error
			status int
		}{
			{"42", nil, 200}, {"x", nil, 400}, {"0", nil, 400}, {"-1", nil, 400},
			{"999", services.ErrExpedienteNotFound, 404},
			{"42", errors.New("private database information"), 500},
		} {
			fake := &fakeParticipants{listErr: tc.err}
			handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, fake)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/expedientes/"+tc.id+"/"+kind, nil))
			if response.Code != tc.status {
				t.Fatalf("%s %s: got %d want %d", kind, tc.id, response.Code, tc.status)
			}
			if tc.status == 200 {
				if strings.TrimSpace(response.Body.String()) != "[]" || fake.expedienteID != 42 || fake.kind != kind {
					t.Fatal("incorrect list routing or empty response")
				}
			}
			if strings.Contains(response.Body.String(), "private database information") {
				t.Fatal("exposed database error")
			}
		}
		fake := &fakeParticipants{list: []services.ParticipantMembershipResponse{{ID: 5, ExpedienteID: 42, Participant: services.ParticipantResponse{ID: 2, Name: "Ana"}}}}
		response := httptest.NewRecorder()
		Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, fake).ServeHTTP(response, httptest.NewRequest("GET", "/expedientes/42/"+kind, nil))
		var result []services.ParticipantMembershipResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result) != 1 {
			t.Fatal("invalid membership response")
		}
		if result[0].ID != 5 || result[0].Participant.ID != 2 {
			t.Fatal("incorrect membership identifiers")
		}
	}
}

func TestFixedParticipantsCannotBeRemovedHTTP(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	for _, kind := range []string{"denunciantes", "denunciados"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("DELETE", "/expedientes/42/"+kind+"/2", nil))
		if w.Code != 404 {
			t.Fatalf("DELETE %s: got %d", kind, w.Code)
		}
	}
}

func (f *fakeDocuments) ListTypes(context.Context) ([]services.DocumentTypeEntry, error) {
	return []services.DocumentTypeEntry{{ID: 1, Code: "OTROS", Name: "OTROS", SortOrder: 1}}, f.err
}

func TestDocumentTypesDictionaryHTTP(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{nil, 200}, {errors.New("database failed"), 500}} {
		w := httptest.NewRecorder()
		Routes(nil, &fakeExpedientes{}, &fakeDocuments{err: tc.err}, &fakeParticipants{}).ServeHTTP(w, httptest.NewRequest("GET", "/document-types", nil))
		if w.Code != tc.status {
			t.Fatalf("got %d want %d", w.Code, tc.status)
		}
		if tc.status == 200 && !strings.Contains(w.Body.String(), "OTROS") {
			t.Fatal("missing dictionary option")
		}
	}
}

func TestInitialParticipantLoadRoutes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, kind := range []string{"denunciantes", "denunciados"} {
			handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{}, RoutesOption{EnableInitialParticipantLoad: enabled})
			w := httptest.NewRecorder()
			body := `{"name":"Initial participant","document_type":"DNI","document_number":"00123456"}`
			if kind == "denunciados" {
				body = `{"name":"Initial participant","document_type":"DNI","document_number":"00123456","registration_number":123}`
			}
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/"+kind, strings.NewReader(body)))
			want := 404
			if enabled {
				want = 201
			}
			if w.Code != want {
				t.Fatalf("%s enabled=%v: got %d want %d", kind, enabled, w.Code, want)
			}
		}
	}
}

func TestParticipantFilterRouting(t *testing.T) {
	fake := &fakeParticipants{}
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, fake)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/denunciados?limit=5&offset=10&q=Garcia&name=Luis&document_type=CE&document_number=00123456&registration_number=123", nil))
	if w.Code != 200 || fake.limit != 5 || fake.offset != 10 || fake.filters.Q != "Garcia" || fake.filters.Name != "Luis" || fake.filters.DocumentType != "CE" || fake.filters.DocumentNumber != "00123456" || fake.filters.RegistrationNumber == nil || *fake.filters.RegistrationNumber != 123 {
		t.Fatal("incorrect participant filters")
	}
	for _, query := range []string{"registration_number=0", "registration_number=abc", "name=Ana&name=Luis"} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/denunciados?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("invalid filter accepted: %s", query)
		}
	}
}

func TestExpedienteFiltersHTTP(t *testing.T) {
	fake := &fakeExpedientes{}
	handler := Routes(nil, fake, nil, nil)
	for _, test := range []struct {
		query string
		code  int
	}{
		{"?status=ABIERTO&location=CONSEJO%20DE%20ETICA&limit=5&offset=10", 200},
		{"?status=OPEN", 400}, {"?location=OTHER", 400}, {"?status=ABIERTO&status=CERRADO", 400}, {"?location=DIRECCION%20DE%20ETICA&location=TRIBUNAL%20DE%20HONOR", 400},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/expedientes"+test.query, nil))
		if w.Code != test.code {
			t.Fatalf("%s: %d", test.query, w.Code)
		}
		if test.code == 200 && (fake.filters.Status != "ABIERTO" || fake.filters.Location != "CONSEJO DE ETICA" || fake.limit != 5 || fake.offset != 10) {
			t.Fatal("filters or pagination lost")
		}
	}
}

func (f *fakeExpedientes) ListByDenunciante(ctx context.Context, id int64, limit, offset int, filter services.ExpedienteFilters) (services.ExpedienteList, error) {
	f.participantID, f.participantRole = id, "denunciantes"
	result, _ := f.List(ctx, limit, offset, filter)
	return result, f.participantErr
}
func (f *fakeExpedientes) ListByDenunciado(ctx context.Context, id int64, limit, offset int, filter services.ExpedienteFilters) (services.ExpedienteList, error) {
	f.participantID, f.participantRole = id, "denunciados"
	result, _ := f.List(ctx, limit, offset, filter)
	return result, f.participantErr
}
func TestParticipantExpedientesHTTP(t *testing.T) {
	for _, role := range []string{"denunciantes", "denunciados"} {
		fake := &fakeExpedientes{}
		handler := Routes(nil, fake, nil, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/"+role+"/42/expedientes?status=CERRADO&location=TRIBUNAL%20DE%20HONOR&limit=5&offset=10", nil))
		if w.Code != 200 || fake.participantID != 42 || fake.participantRole != role || fake.filters.Status != "CERRADO" || fake.filters.Location != "TRIBUNAL DE HONOR" || fake.limit != 5 || fake.offset != 10 {
			t.Fatalf("participant expediente route failed: %s %d", role, w.Code)
		}
		for _, suffix := range []string{"0/expedientes", "x/expedientes", "42/expedientes?limit=0", "42/expedientes?offset=-1", "42/expedientes?status=OPEN", "42/expedientes?location=OTHER", "42/expedientes?status=ABIERTO&status=CERRADO", "42/expedientes?limit=1&limit=2"} {
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/"+role+"/"+suffix, nil))
			if w.Code != 400 {
				t.Fatalf("invalid %s: %d", suffix, w.Code)
			}
		}
		fake.participantErr = services.ErrParticipantNotFound
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/"+role+"/999/expedientes", nil))
		if w.Code != 404 {
			t.Fatal("missing participant must return 404")
		}
	}
}

func TestOpeningDateFiltersHTTP(t *testing.T) {
	for _, path := range []string{"/expedientes", "/denunciantes/42/expedientes", "/denunciados/42/expedientes"} {
		fake := &fakeExpedientes{}
		handler := Routes(nil, fake, nil, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path+"?from=2026-01-01&to=2026-12-31&status=ABIERTO&limit=10&offset=20", nil))
		if w.Code != 200 || fake.filters.From != "2026-01-01" || fake.filters.To != "2026-12-31" || fake.filters.Status != "ABIERTO" || fake.limit != 10 || fake.offset != 20 {
			t.Fatalf("date filters lost on %s: %d", path, w.Code)
		}
		for _, query := range []string{"?from=2026-02-30", "?to=30-10-2026", "?from=2026-12-31&to=2026-01-01", "?from=2026-01-01&from=2026-02-01", "?to=2026-01-01&to=2026-02-01"} {
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", path+query, nil))
			if w.Code != 400 {
				t.Fatalf("invalid range on %s%s: %d", path, query, w.Code)
			}
		}
	}
}

func (f *fakeExpedientes) UpdateDescription(_ context.Context, id int64, input services.UpdateExpedienteDescriptionInput) (services.ExpedienteResponse, error) {
	f.descriptionID, f.descriptionInput = id, input
	if input.Description == nil {
		return services.ExpedienteResponse{}, services.ErrExpedienteInvalid
	}
	return services.ExpedienteResponse{ID: id, Description: *input.Description}, f.statusErr
}

func TestUpdateExpedienteDescriptionHTTP(t *testing.T) {
	fake := &fakeExpedientes{}
	handler := Routes(nil, fake, &fakeDocuments{}, &fakeParticipants{})
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/expedientes/42/description", `{"description":"Nueva descripcion"}`, 200},
		{"/expedientes/42/description", `{"description":""}`, 200},
		{"/expedientes/x/description", `{"description":"Texto"}`, 400},
		{"/expedientes/0/description", `{"description":"Texto"}`, 400},
		{"/expedientes/42/description", `{}`, 400},
		{"/expedientes/42/description", `{"description":null}`, 400},
		{"/expedientes/42/description", `{"description":42}`, 400},
		{"/expedientes/42/description", `{"description":"Texto","status":"CERRADO"}`, 400},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("PATCH", tc.path, strings.NewReader(tc.body)))
		if response.Code != tc.status {
			t.Errorf("%s %s got %d: %s", tc.path, tc.body, response.Code, response.Body.String())
		}
		if tc.status == 200 && (fake.descriptionID != 42 || !strings.Contains(response.Body.String(), `"description":`)) {
			t.Fatal("description not forwarded or returned")
		}
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{services.ErrExpedienteNotFound, 404}, {services.ErrFolderClosed, 409}, {errors.New("private database details"), 500}} {
		fake.statusErr = tc.err
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("PATCH", "/expedientes/42/description", strings.NewReader(`{"description":"Texto"}`)))
		if response.Code != tc.status || strings.Contains(response.Body.String(), "private database details") {
			t.Fatalf("unexpected error response: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestRetiredExpedienteStateRoutes(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	for _, tc := range []struct{ method, path string }{
		{"PATCH", "/expedientes/42/status"}, {"PATCH", "/expedientes/42/location"},
		{"GET", "/expediente-statuses"}, {"GET", "/expediente-locations"},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		if w.Code != 404 {
			t.Fatalf("retired route %s still available: %d", tc.path, w.Code)
		}
	}
	for _, path := range []string{"/expedientes/42/folders/0/status", "/expedientes/42/folders/x/status", "/expedientes/42/folders/0/location", "/expedientes/42/folders/x/location"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("PATCH", path, strings.NewReader(`{}`)))
		if w.Code != 400 {
			t.Fatalf("invalid folder ID %s: %d", path, w.Code)
		}
	}
}
