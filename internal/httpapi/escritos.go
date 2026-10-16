package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"etica_back/internal/services"
)

type EscritoAPI interface {
	Create(context.Context, services.CreateEscritoInput) (services.EscritoResponse, error)
	List(context.Context, string, int, int) (services.EscritoPage, error)
	UpdateStatus(context.Context, int64, string) (services.EscritoResponse, error)
}

func registerEscritos(mux *http.ServeMux, service EscritoAPI) {
	mux.HandleFunc("POST /escritos", func(w http.ResponseWriter, r *http.Request) {
		var input services.CreateEscritoInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.Create(r.Context(), input)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusCreated, result)
	})
	mux.HandleFunc("GET /escritos", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := escritoPage(r)
		if !ok {
			failure(w, services.ErrEscritoInvalid)
			return
		}
		statusValues, hasStatus := r.URL.Query()["status"]
		if hasStatus && len(statusValues) != 1 {
			failure(w, services.ErrEscritoInvalid)
			return
		}
		status := ""
		if hasStatus {
			status = statusValues[0]
		}
		result, err := service.List(r.Context(), status, limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("PATCH /escritos/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrEscritoInvalid)
			return
		}
		var input struct {
			Status string `json:"status"`
		}
		if !decode(w, r, &input) {
			return
		}
		result, err := service.UpdateStatus(r.Context(), id, input.Status)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
}

func escritoPage(r *http.Request) (int, int, bool) {
	limit, offset := 20, 0
	if values, ok := r.URL.Query()["limit"]; ok {
		if len(values) != 1 {
			return 0, 0, false
		}
		v, err := strconv.Atoi(values[0])
		if err != nil {
			return 0, 0, false
		}
		limit = v
	}
	if values, ok := r.URL.Query()["offset"]; ok {
		if len(values) != 1 {
			return 0, 0, false
		}
		v, err := strconv.Atoi(values[0])
		if err != nil {
			return 0, 0, false
		}
		offset = v
	}
	return limit, offset, limit >= 1 && limit <= 100 && offset >= 0
}
