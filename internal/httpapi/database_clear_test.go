package httpapi

import (
	"context"
	"errors"

	"net/http/httptest"
	"testing"

	"etica_back/internal/services"
)

type fakeDatabaseClear struct {
	calls int
	err   error
}

func (f *fakeDatabaseClear) Clear(context.Context) error { f.calls++; return f.err }

func TestDatabaseClearRoute(t *testing.T) {
	for _, test := range []struct {
		name, method  string
		err           error
		status, calls int
	}{
		{"success", "DELETE", nil, 200, 1},
		{"disabled", "DELETE", services.ErrDatabaseClearDisabled, 404, 1},
		{"failure", "DELETE", errors.New("database error"), 500, 1},
		{"get cannot delete", "GET", nil, 405, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			clear := &fakeDatabaseClear{err: test.err}
			h := Routes(nil, nil, nil, nil, RoutesOption{DatabaseClear: clear})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(test.method, "/dev/database/clear", nil))
			if w.Code != test.status || clear.calls != test.calls {
				t.Fatalf("status=%d calls=%d", w.Code, clear.calls)
			}
		})
	}
	// A disabled service must refuse before accessing a database connection.
	if err := services.NewDatabaseClearService(nil, false).Clear(context.Background()); !errors.Is(err, services.ErrDatabaseClearDisabled) {
		t.Fatal(err)
	}

}
