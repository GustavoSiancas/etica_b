package httpapi

import (
	"context"
	"errors"
	"etica_back/internal/services"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeFolderTypes struct {
	DocumentAPI
	input  services.CreateFolderTypeInput
	err    error
	called bool
}

func (f *fakeFolderTypes) CreateFolderType(_ context.Context, input services.CreateFolderTypeInput) (services.FolderTypeResponse, error) {
	f.input = input
	f.called = true
	return services.FolderTypeResponse{ID: 3, Name: "EXPEDIENTE ESPECIAL"}, f.err
}

func TestCreateFolderTypeHTTP(t *testing.T) {
	for _, tc := range []struct {
		body   string
		err    error
		status int
		called bool
	}{
		{`{"name":"Expediente especial"}`, nil, 201, true},
		{`{"name":""}`, services.ErrFolderTypeInvalid, 400, true},
		{`{}`, services.ErrFolderTypeInvalid, 400, true},
		{`{"name":null}`, services.ErrFolderTypeInvalid, 400, true},
		{`{"name":42}`, nil, 400, false},
		{`{"name":"Nuevo","extra":true}`, nil, 400, false},
		{`{"name":"Nuevo"} {}`, nil, 400, false},
		{`{"name":"EXPEDIENTE PRINCIPAL"}`, services.ErrFolderTypeDuplicate, 409, true},
		{`{"name":"Nuevo"}`, errors.New("private database details"), 500, true},
	} {
		fake := &fakeFolderTypes{err: tc.err}
		handler := Routes(nil, &fakeExpedientes{}, fake, &fakeParticipants{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/folder-types", strings.NewReader(tc.body)))
		if w.Code != tc.status || fake.called != tc.called || strings.Contains(w.Body.String(), "private database details") {
			t.Fatalf("body %s got %d called %v: %s", tc.body, w.Code, fake.called, w.Body.String())
		}
		if tc.status == 201 && (fake.input.Name != "Expediente especial" || !strings.Contains(w.Body.String(), `"id":3`)) {
			t.Fatal("input or created type not returned")
		}
	}
}
