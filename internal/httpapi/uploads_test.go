package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"etica_back/internal/services"
)

type fakeDemoData struct {
	enabled bool
	called  bool
}

func (f *fakeDemoData) Enabled() bool { return f.enabled }
func (f *fakeDemoData) Reset(context.Context) (services.DemoDataSummary, error) {
	f.called = true
	return services.DemoDataSummary{Expedientes: 3, Denunciantes: 104, Denunciados: 154, Documents: 45, Escritos: 6}, nil
}

func TestInvalidRequestsAndCORS(t *testing.T) {
	handler := Routes(nil, nil, nil, &fakeParticipants{})
	for _, test := range []struct{ method, path, body string }{
		{"POST", "/documents/invalid/upload-url", `{"content_type":"image/png"}`},
		{"POST", "/documents/1/upload-url", `{"unknown":"value"}`},
		{"POST", "/documents/1/upload-url", `{"content_type":"image/png"} {}`},
		{"PATCH", "/documents/0/file-url", `{"file_url":"https://example.com/x"}`},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 400 {
			t.Errorf("%s: got %d", test.path, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodOptions, "/documents/1/upload-url", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 204 || response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("frontend preflight failed")
	}
	request.Header.Set("Origin", "https://unrelated.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("wildcard CORS should allow every origin")
	}
}

func TestDemoDataResetRouteIsOptIn(t *testing.T) {
	disabled := &fakeDemoData{}
	handler := Routes(nil, nil, nil, nil, RoutesOption{DemoData: disabled})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dev/demo-data/reset", nil))
	if response.Code != http.StatusNotFound || disabled.called {
		t.Fatal("disabled demo reset route should not be registered")
	}

	enabled := &fakeDemoData{enabled: true}
	handler = Routes(nil, nil, nil, nil, RoutesOption{DemoData: enabled})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dev/demo-data/reset", nil))
	if response.Code != http.StatusOK || !enabled.called || !strings.Contains(response.Body.String(), `"documents":45`) {
		t.Fatal("enabled demo reset route did not reload the fixture")
	}
}
