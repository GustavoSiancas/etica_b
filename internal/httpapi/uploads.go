package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"etica_back/internal/services"
	"io"
	"net/http"
	"strconv"
)

type RoutesOption struct {
	Escrito  EscritoAPI
	DemoData DemoDataAPI
}

type DemoDataAPI interface {
	Enabled() bool
	Reset(context.Context) (services.DemoDataSummary, error)
}

func Routes(service *services.UploadService, expedientes ExpedienteAPI, documents DocumentAPI, participants ParticipantAPI, options ...RoutesOption) http.Handler {
	mux := http.NewServeMux()
	registerSwagger(mux)
	registerExpedientes(mux, expedientes, documents, participants)
	if len(options) > 0 {
		if options[0].Escrito != nil {
			registerEscritos(mux, options[0].Escrito)
		}
		if demo := options[0].DemoData; demo != nil {
			mux.HandleFunc("GET /dev/demo-data/reset", func(w http.ResponseWriter, r *http.Request) {
				if !demo.Enabled() {
					failure(w, services.ErrDemoResetDisabled)
					return
				}
				result, err := demo.Reset(r.Context())
				if err != nil {
					failure(w, err)
					return
				}
				respond(w, http.StatusOK, result)
			})
		}
	}
	mux.HandleFunc("POST /documents/{id}/upload-url", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrInvalid)
			return
		}
		var body struct {
			ContentType string `json:"content_type"`
		}
		if !decode(w, r, &body) {
			return
		}
		result, err := service.Sign(r.Context(), id, body.ContentType)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, result)
	})
	mux.HandleFunc("POST /escritos/{id}/upload-url", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrInvalid)
			return
		}
		var body struct {
			ContentType string `json:"content_type"`
		}
		if !decode(w, r, &body) {
			return
		}
		result, err := service.SignEscrito(r.Context(), id, body.ContentType)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, result)
	})
	mux.HandleFunc("PATCH /escritos/{id}/file-url", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrInvalid)
			return
		}
		var body struct {
			FileURL string `json:"file_url"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := service.ConfirmEscrito(r.Context(), id, body.FileURL); err != nil {
			failure(w, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"file_url": body.FileURL})
	})
	mux.HandleFunc("PATCH /documents/{id}/file-url", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			failure(w, services.ErrInvalid)
			return
		}
		var body struct {
			FileURL string `json:"file_url"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := service.Confirm(r.Context(), id, body.FileURL); err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, map[string]string{"file_url": body.FileURL})
	})
	return requestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "3600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	}))
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		failure(w, services.ErrInvalid)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, services.ErrInvalid)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, err error) {
	if out, ok := w.(*loggedResponse); ok {
		out.err = err
	}
	status := 500
	message := "internal server error"
	switch {
	case errors.Is(err, services.ErrParticipantInvalid):
		status = 400
		message = "invalid participant name, identifier or pagination"
	case errors.Is(err, services.ErrParticipantNotFound):
		status = 404
		message = "participant not found"
	case errors.Is(err, services.ErrParticipantDuplicate):
		status = 409
		message = "participant already belongs to expediente"
	case errors.Is(err, services.ErrMembershipNotFound):
		status = 404
		message = "active participant association not found"
	case errors.Is(err, services.ErrDocumentInvalid):
		status = 400
		message = "invalid document fields"
	case errors.Is(err, services.ErrExpedienteNotFound):
		status = 404
		message = "expediente not found"
	case errors.Is(err, services.ErrSubstageNotFound):
		status = 400
		message = "document substage not found or inactive"
	case errors.Is(err, services.ErrExpedienteInvalid):
		status = 400
		message = "limit must be 1 to 100 and offset nonnegative"
	case errors.Is(err, services.ErrExpedienteDuplicate):
		status = 409
		message = "expediente number already exists"
	case errors.Is(err, services.ErrInvalid):
		status = 400
		message = "invalid request body, image type, size or file URL"
	case errors.Is(err, services.ErrNotFound):
		status = 404
		message = "document not found"
	case errors.Is(err, services.ErrStorage):
		status = 502
		message = "uploaded object could not be verified"
	case errors.Is(err, services.ErrEscritoInvalid):
		status = 400
		message = "invalid escrito fields or pagination"
	case errors.Is(err, services.ErrEscritoNotFound):
		status = 404
		message = "escrito not found"
	case errors.Is(err, services.ErrEscritoAlreadyAttended):
		status = 409
		message = "escrito is already attended"
	case errors.Is(err, services.ErrDemoResetDisabled):
		status = 404
		message = "not found"
	}
	respond(w, status, map[string]string{"error": message})
}
