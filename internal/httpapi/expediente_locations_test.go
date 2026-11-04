package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"etica_back/internal/services"
)

func TestFolderLocationsHTTP(t *testing.T) {
	fake := &fakeDocuments{}
	handler := Routes(nil, &fakeExpedientes{}, fake, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/folder-locations", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "DIRECCION DE ETICA") || !strings.Contains(w.Body.String(), "CONSEJO DE ETICA") || !strings.Contains(w.Body.String(), "TRIBUNAL DE HONOR") {
		t.Fatal("missing locations")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("PATCH", "/expedientes/42/folders/42/location", strings.NewReader(`{"location":"CONSEJO DE ETICA"}`)))
	if w.Code != 200 || fake.locationID != 42 || fake.locationInput.Location != services.UbicacionConsejo {
		t.Fatal("location endpoint failed")
	}
	for _, test := range []struct{ path, body string }{
		{"/expedientes/0/folders/42/location", `{"location":"CONSEJO DE ETICA"}`},
		{"/expedientes/x/folders/42/location", `{"location":"CONSEJO DE ETICA"}`},
		{"/expedientes/42/folders/42/location", `{"location":"CONSEJO DE ETICA","extra":1}`},
	} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("PATCH", test.path, strings.NewReader(test.body)))
		if w.Code != 400 {
			t.Fatalf("invalid location request: %d", w.Code)
		}
	}
	fake.statusErr = services.ErrExpedienteNotFound
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("PATCH", "/expedientes/42/folders/42/location", strings.NewReader(`{"location":"CONSEJO DE ETICA"}`)))
	if w.Code != 404 {
		t.Fatalf("missing expediente location update: %d", w.Code)
	}
}
