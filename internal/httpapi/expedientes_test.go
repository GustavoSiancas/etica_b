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
	createErr     error
	limit, offset int
	statusInput   services.UpdateExpedienteStatusInput
	statusID      int64
	statusErr     error
}
type fakeDocuments struct {
	create       services.DocumentResponse
	list         []services.DocumentResponse
	stages       []services.DocumentStageCatalog
	err          error
	expedienteID int64
}

func (f *fakeDocuments) ListStages(context.Context) ([]services.DocumentStageCatalog, error) {
	return f.stages, f.err
}

type fakeParticipants struct {
	list         []services.ParticipantMembershipResponse
	listErr      error
	expedienteID int64
	kind         string
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

func (*fakeParticipants) CreateDenunciante(_ context.Context, name string) (services.ParticipantResponse, error) {
	return services.ParticipantResponse{ID: 1, Name: name}, nil
}
func (*fakeParticipants) CreateDenunciado(_ context.Context, name string) (services.ParticipantResponse, error) {
	return services.ParticipantResponse{ID: 2, Name: name}, nil
}
func (*fakeParticipants) ListDenunciantes(context.Context, int, int) ([]services.ParticipantResponse, int64, error) {
	return []services.ParticipantResponse{}, 0, nil
}
func (*fakeParticipants) ListDenunciados(context.Context, int, int) ([]services.ParticipantResponse, int64, error) {
	return []services.ParticipantResponse{}, 0, nil
}
func (*fakeParticipants) AttachDenunciante(_ context.Context, expedienteID, participantID int64) (services.ParticipantMembershipResponse, error) {
	return services.ParticipantMembershipResponse{ID: 5, ExpedienteID: expedienteID, Participant: services.ParticipantResponse{ID: participantID, Name: "Ana"}}, nil
}
func (*fakeParticipants) AttachDenunciado(_ context.Context, expedienteID, participantID int64) (services.ParticipantMembershipResponse, error) {
	return services.ParticipantMembershipResponse{ID: 6, ExpedienteID: expedienteID, Participant: services.ParticipantResponse{ID: participantID, Name: "Luis"}}, nil
}
func (*fakeParticipants) RemoveDenunciante(context.Context, int64, int64) error { return nil }
func (*fakeParticipants) RemoveDenunciado(context.Context, int64, int64) error  { return nil }

func (f *fakeDocuments) Create(_ context.Context, id int64, input services.CreateDocumentInput) (services.DocumentResponse, error) {
	f.expedienteID = id
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
	return services.ExpedienteResponse{ID: 1, Number: "1-2026", Code: "1-2026", InitType: &initType, Status: "OPEN"}, f.createErr
}
func (f *fakeExpedientes) ListInitTypes(context.Context) ([]services.ExpedienteInitTypeResponse, error) {
	return []services.ExpedienteInitTypeResponse{{Value: "DENUNCIA_DE_OFICIO", Label: "DENUNCIA DE OFICIO"}}, nil
}
func (f *fakeExpedientes) ListStatuses(context.Context) ([]services.ExpedienteStatusResponse, error) {
	return []services.ExpedienteStatusResponse{{Value: "OPEN", Label: "Abierto"}, {Value: "IN_PROGRESS", Label: "En proceso"}, {Value: "CLOSED", Label: "Cerrado"}}, nil
}
func (f *fakeExpedientes) List(_ context.Context, limit, offset int) (services.ExpedienteList, error) {
	f.limit = limit
	f.offset = offset
	return services.ExpedienteList{Data: []services.ExpedienteResponse{}, Limit: limit, Offset: offset}, nil
}
func (f *fakeExpedientes) UpdateStatus(_ context.Context, id int64, input services.UpdateExpedienteStatusInput) (services.ExpedienteResponse, error) {
	f.statusID, f.statusInput = id, input
	return services.ExpedienteResponse{ID: id, Status: input.Status}, f.statusErr
}

func TestCreateExpedienteHTTP(t *testing.T) {
	for _, test := range []struct {
		body   string
		err    error
		status int
	}{
		{`{"name":"Caso de prueba","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_OFICIO"}`, nil, 201},
		{`{"name":"Caso de prueba","init_type":"DENUNCIA_DE_OFICIO"}`, services.ErrExpedienteDuplicate, 409},
		{`{"number":""}`, nil, 400},
		{`{"name":"Caso","init_type":"UNKNOWN"}`, services.ErrExpedienteInvalid, 400},
		{`{"number":"EXP-1","status":"CLOSED"}`, nil, 400},
		{`{"name":"Caso","init_type":"DENUNCIA_DE_OFICIO"}`, errors.New("private database information"), 500},
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
			var data services.ExpedienteResponse
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || data.Status != "OPEN" || data.ID != 1 || data.InitType == nil || *data.InitType != "DENUNCIA_DE_OFICIO" {
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

func TestUpdateExpedienteStatusHTTP(t *testing.T) {
	fake := &fakeExpedientes{}
	handler := Routes(nil, fake, &fakeDocuments{}, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("PATCH", "/expedientes/42/status", strings.NewReader(`{"status":"EN INVESTIGACION"}`)))
	if response.Code != 200 || fake.statusID != 42 || fake.statusInput.Status != "EN INVESTIGACION" || !strings.Contains(response.Body.String(), `"status":"EN INVESTIGACION"`) {
		t.Fatal("expediente status update endpoint failed")
	}
	for _, test := range []struct {
		path, body string
		status     int
	}{
		{"/expedientes/x/status", `{"status":"OPEN"}`, 400},
		{"/expedientes/0/status", `{"status":"OPEN"}`, 400},
		{"/expedientes/42/status", `{"status":"OPEN","extra":true}`, 400},
	} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("PATCH", test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Errorf("%s got %d want %d", test.path, response.Code, test.status)
		}
	}
	fake.statusErr = services.ErrExpedienteNotFound
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("PATCH", "/expedientes/999/status", strings.NewReader(`{"status":"CLOSED"}`)))
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

func TestListExpedienteStatuses(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/expediente-statuses", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"value":"OPEN"`) || !strings.Contains(response.Body.String(), `"value":"IN_PROGRESS"`) || !strings.Contains(response.Body.String(), `"value":"CLOSED"`) {
		t.Fatal("expediente status catalog endpoint failed")
	}
}

func TestDocumentEndpoints(t *testing.T) {
	input := services.DocumentResponse{ID: 7, ExpedienteID: 42, Title: "Resolución", Status: "DRAFT"}
	fake := &fakeDocuments{create: input, list: []services.DocumentResponse{input}}
	handler := Routes(nil, &fakeExpedientes{}, fake, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/expedientes/42/documents", strings.NewReader(`{"title":"Resolución","substage_id":3}`)))
	if response.Code != 201 || fake.expedienteID != 42 {
		t.Fatal("document creation endpoint failed")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/expedientes/42/documents", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Resolución") || fake.expedienteID != 42 {
		t.Fatal("document list endpoint failed")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/expedientes/no/documents", strings.NewReader(`{}`)))
	if response.Code != 400 {
		t.Fatal("invalid expediente path accepted")
	}
	fake.err = services.ErrExpedienteNotFound
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/expedientes/999/documents", nil))
	if response.Code != 404 {
		t.Fatal("missing expediente response should be 404")
	}
}

func TestListDocumentStagesHTTP(t *testing.T) {
	fake := &fakeDocuments{stages: []services.DocumentStageCatalog{{ID: 1, Code: "CALIFICACION_DENUNCIA", Name: "Calificación de la denuncia.", SortOrder: 1, Substages: []services.DocumentSubstageEntry{{ID: 3, Code: "CALIFICACION", Name: "Calificación de la denuncia.", SortOrder: 1}}}}}
	handler := Routes(nil, &fakeExpedientes{}, fake, &fakeParticipants{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/document-stages", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"code":"CALIFICACION_DENUNCIA"`) || !strings.Contains(response.Body.String(), `"code":"CALIFICACION"`) {
		t.Fatal("document stage catalog endpoint failed")
	}
}

func TestParticipantEndpoints(t *testing.T) {
	handler := Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, &fakeParticipants{})
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/denunciantes", `{"name":"Ana"}`, 201},
		{"GET", "/denunciantes?limit=10", ``, 200},
		{"POST", "/denunciados", `{"name":"Luis"}`, 201},
		{"GET", "/denunciados", ``, 200},
		{"POST", "/expedientes/4/denunciantes", `{"denunciante_id":1}`, 201},
		{"POST", "/expedientes/4/denunciados", `{"denunciado_id":2}`, 201},
		{"POST", "/expedientes/x/denunciantes", `{"denunciante_id":1}`, 400},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Errorf("%s %s: got %d want %d", test.method, test.path, response.Code, test.status)
		}
	}
}

func TestActiveParticipantsByExpedienteHTTP(t *testing.T) {
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
		fake := &fakeParticipants{list: []services.ParticipantMembershipResponse{{ID: 5, ExpedienteID: 42, Participant: services.ParticipantResponse{ID: 2, Name: "Ana"}, IsActive: true}}}
		response := httptest.NewRecorder()
		Routes(nil, &fakeExpedientes{}, &fakeDocuments{}, fake).ServeHTTP(response, httptest.NewRequest("GET", "/expedientes/42/"+kind, nil))
		var result []services.ParticipantMembershipResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result) != 1 {
			t.Fatal("invalid membership response")
		}
		if result[0].ID != 5 || result[0].Participant.ID != 2 || !result[0].IsActive {
			t.Fatal("incorrect membership identifiers")
		}
	}
}
