package httpapi

import (
	"context"
	"etica_back/internal/services"
	"net/http"
	"strconv"
)

type ParticipantAPI interface {
	ListDenunciantesByExpediente(context.Context, int64) ([]services.ParticipantMembershipResponse, error)
	ListDenunciadosByExpediente(context.Context, int64) ([]services.ParticipantMembershipResponse, error)
	CreateDenunciante(context.Context, string) (services.ParticipantResponse, error)
	CreateDenunciado(context.Context, string) (services.ParticipantResponse, error)
	ListDenunciantes(context.Context, int, int) ([]services.ParticipantResponse, int64, error)
	ListDenunciados(context.Context, int, int) ([]services.ParticipantResponse, int64, error)
	AttachDenunciante(context.Context, int64, int64) (services.ParticipantMembershipResponse, error)
	AttachDenunciado(context.Context, int64, int64) (services.ParticipantMembershipResponse, error)
	RemoveDenunciante(context.Context, int64, int64) error
	RemoveDenunciado(context.Context, int64, int64) error
}

func registerParticipants(mux *http.ServeMux, service ParticipantAPI) {
	mux.HandleFunc("GET /expedientes/{id}/denunciados", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		result, err := service.ListDenunciadosByExpediente(r.Context(), id)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /expedientes/{id}/denunciantes", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		result, err := service.ListDenunciantesByExpediente(r.Context(), id)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /denunciantes", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateDenunciante(r.Context(), input.Name)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /denunciados", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateDenunciado(r.Context(), input.Name)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusCreated, result)
	})
	mux.HandleFunc("GET /denunciantes", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := participantPage(r)
		if !ok {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		data, total, err := service.ListDenunciantes(r.Context(), limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, map[string]any{"data": data, "total": total, "limit": limit, "offset": offset})
	})
	mux.HandleFunc("GET /denunciados", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := participantPage(r)
		if !ok {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		data, total, err := service.ListDenunciados(r.Context(), limit, offset)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, map[string]any{"data": data, "total": total, "limit": limit, "offset": offset})
	})
	mux.HandleFunc("POST /expedientes/{id}/denunciantes", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		var input struct {
			ID int64 `json:"denunciante_id"`
		}
		if !decode(w, r, &input) {
			return
		}
		result, err := service.AttachDenunciante(r.Context(), id, input.ID)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 201, result)
	})
	mux.HandleFunc("POST /expedientes/{id}/denunciados", func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePositiveID(r.PathValue("id"))
		if err != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		var input struct {
			ID int64 `json:"denunciado_id"`
		}
		if !decode(w, r, &input) {
			return
		}
		result, err := service.AttachDenunciado(r.Context(), id, input.ID)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 201, result)
	})
	mux.HandleFunc("DELETE /expedientes/{id}/denunciantes/{participant_id}", func(w http.ResponseWriter, r *http.Request) {
		expedienteID, err := parsePositiveID(r.PathValue("id"))
		participantID, idErr := parsePositiveID(r.PathValue("participant_id"))
		if err != nil || idErr != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		if err = service.RemoveDenunciante(r.Context(), expedienteID, participantID); err != nil {
			failure(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /expedientes/{id}/denunciados/{participant_id}", func(w http.ResponseWriter, r *http.Request) {
		expedienteID, err := parsePositiveID(r.PathValue("id"))
		participantID, idErr := parsePositiveID(r.PathValue("participant_id"))
		if err != nil || idErr != nil {
			failure(w, services.ErrParticipantInvalid)
			return
		}
		if err = service.RemoveDenunciado(r.Context(), expedienteID, participantID); err != nil {
			failure(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func participantPage(r *http.Request) (int, int, bool) {
	limit, offset := 20, 0
	var err error
	if raw, ok := r.URL.Query()["limit"]; ok {
		if len(raw) != 1 {
			return 0, 0, false
		}
		limit, err = strconv.Atoi(raw[0])
		if err != nil {
			return 0, 0, false
		}
	}
	if raw, ok := r.URL.Query()["offset"]; ok {
		if len(raw) != 1 {
			return 0, 0, false
		}
		offset, err = strconv.Atoi(raw[0])
		if err != nil {
			return 0, 0, false
		}
	}
	return limit, offset, limit >= 1 && limit <= 100 && offset >= 0
}
func parsePositiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, services.ErrParticipantInvalid
	}
	return id, nil
}
