package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"etica_back/internal/services"
)

type ExpedienteAPI interface {
	ListFolderHistory(context.Context, int64, int64, int, int) (services.ExpedienteHistoryPage, error)
	ListHistory(context.Context, int64, int, int) (services.ExpedienteHistoryPage, error)
	UpdateStatus(context.Context, int64, services.UpdateExpedienteStatusInput) (services.ExpedienteResponse, error)
	UpdateDescription(context.Context, int64, services.UpdateExpedienteDescriptionInput) (services.ExpedienteResponse, error)
	ListByDenunciante(context.Context, int64, int, int, services.ExpedienteFilters) (services.ExpedienteList, error)
	ListByDenunciado(context.Context, int64, int, int, services.ExpedienteFilters) (services.ExpedienteList, error)
	Create(context.Context, services.CreateExpedienteInput) (services.ExpedienteResponse, error)
	List(context.Context, int, int, ...services.ExpedienteFilters) (services.ExpedienteList, error)
	ListInitTypes(context.Context) ([]services.ExpedienteInitTypeResponse, error)
}

func registerExpedientes(mux *http.ServeMux, service ExpedienteAPI) {
	registerParticipantExpedientes(mux, service)
	mux.HandleFunc("GET /expedientes/{id}/folders/{folderID}/history", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		folderID, err := parsePositiveID(r.PathValue("folderID"))
		if err != nil {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		limit, offset, ok := participantPage(r)
		if !ok {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		result, err := service.ListFolderHistory(r.Context(), id, folderID, limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /expedientes/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		limit, offset, ok := participantPage(r)
		if !ok {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		result, err := service.ListHistory(r.Context(), id, limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("PATCH /expedientes/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
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
	mux.HandleFunc("PATCH /expedientes/{id}/description", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrExpedienteInvalid)
			return
		}
		var input services.UpdateExpedienteDescriptionInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.UpdateDescription(r.Context(), id, input)
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
		filter, err := expedienteFilters(r)
		if err != nil {
			failure(w, err)
			return
		}
		result, err := service.List(r.Context(), limit, offset, filter)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
}

func expedienteFilters(r *http.Request) (services.ExpedienteFilters, error) {
	filter := services.ExpedienteFilters{}
	for key, target := range map[string]*string{"status": &filter.Status, "location": &filter.Location, "from": &filter.From, "to": &filter.To} {
		if values, ok := r.URL.Query()[key]; ok {
			if len(values) != 1 {
				return filter, services.ErrExpedienteInvalid
			}
			*target = values[0]
		}
	}
	if !services.ValidExpedienteFilters(filter) {
		return filter, services.ErrExpedienteInvalid
	}
	return filter, nil
}
func registerParticipantExpedientes(mux *http.ServeMux, service ExpedienteAPI) {
	// Resolve methods inside the handler so optional nil services used by other
	// isolated HTTP route tests are never dereferenced during registration.
	for _, role := range []string{"denunciantes", "denunciados"} {
		mux.HandleFunc("GET /"+role+"/{id}/expedientes", func(w http.ResponseWriter, r *http.Request) {
			id, err := parsePositiveID(r.PathValue("id"))
			if err != nil {
				failure(w, services.ErrParticipantInvalid)
				return
			}
			limit, offset, ok := participantPage(r)
			if !ok {
				failure(w, services.ErrExpedienteInvalid)
				return
			}
			filter, err := expedienteFilters(r)
			if err != nil {
				failure(w, err)
				return
			}
			var result services.ExpedienteList
			if role == "denunciantes" {
				result, err = service.ListByDenunciante(r.Context(), id, limit, offset, filter)
			} else {
				result, err = service.ListByDenunciado(r.Context(), id, limit, offset, filter)
			}
			if err != nil {
				failure(w, err)
				return
			}
			respond(w, http.StatusOK, result)
		})
	}
}
