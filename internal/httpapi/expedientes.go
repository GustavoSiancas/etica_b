package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"etica_back/internal/services"
)

type ExpedienteAPI interface {
	Create(context.Context, services.CreateExpedienteInput) (services.ExpedienteResponse, error)
	List(context.Context, int, int) (services.ExpedienteList, error)
	ListInitTypes(context.Context) ([]services.ExpedienteInitTypeResponse, error)
	ListStatuses(context.Context) ([]services.ExpedienteStatusResponse, error)
	UpdateStatus(context.Context, int64, services.UpdateExpedienteStatusInput) (services.ExpedienteResponse, error)
}

type DocumentAPI interface {
	ListStages(context.Context) ([]services.DocumentStageCatalog, error)
	Create(context.Context, int64, services.CreateDocumentInput) (services.DocumentResponse, error)
	ListByExpediente(context.Context, int64) ([]services.DocumentResponse, error)
}

func registerExpedientes(mux *http.ServeMux, service ExpedienteAPI, documents DocumentAPI, participants ParticipantAPI) {
	registerParticipants(mux, participants)
	mux.HandleFunc("GET /document-stages", func(w http.ResponseWriter, r *http.Request) {
		result, err := documents.ListStages(r.Context())
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("PATCH /expedientes/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		var input services.UpdateExpedienteStatusInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.UpdateStatus(r.Context(), id, input)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /expediente-statuses", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListStatuses(r.Context())
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /expedientes/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			failure(w, services.ErrDocumentInvalid)
			return
		}
		var input services.CreateDocumentInput
		if !decode(w, r, &input) {
			return
		}
		result, err := documents.Create(r.Context(), id, input)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusCreated, result)
	})
	mux.HandleFunc("GET /expedientes/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			failure(w, services.ErrDocumentInvalid)
			return
		}
		result, err := documents.ListByExpediente(r.Context(), id)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /tipos-inicio", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListInitTypes(r.Context())
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /expedientes", func(w http.ResponseWriter, r *http.Request) {
		var input services.CreateExpedienteInput
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
	mux.HandleFunc("GET /expedientes", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 20, 0
		var err error
		if value, ok := r.URL.Query()["limit"]; ok {
			if len(value) != 1 {
				failure(w, services.ErrExpedienteInvalid)
				return
			}
			limit, err = strconv.Atoi(value[0])
			if err != nil {
				failure(w, services.ErrExpedienteInvalid)
				return
			}
		}
		if value, ok := r.URL.Query()["offset"]; ok {
			if len(value) != 1 {
				failure(w, services.ErrExpedienteInvalid)
				return
			}
			offset, err = strconv.Atoi(value[0])
			if err != nil {
				failure(w, services.ErrExpedienteInvalid)
				return
			}
		}
		if limit < 1 || limit > 100 || offset < 0 {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		result, err := service.List(r.Context(), limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
}
