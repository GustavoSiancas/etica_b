package httpapi

import (
	"etica_back/internal/services"
	"net/http"
	"strconv"
)

// Catalogue creation is intentionally open until administrator authentication is added.
func registerFolderTypes(mux *http.ServeMux, service DocumentAPI) {
	mux.HandleFunc("GET /expedientes/{expedienteID}/folders/{folderID}/status/closed", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		id, err := parsePositiveID(r.PathValue("folderID"))
		if err != nil {
			failure(w, services.ErrFolderInvalid)
			return
		}
		result, err := service.GetFolderCloseStatus(r.Context(), e, id)
		w.Header().Set("Cache-Control", "no-store")
		documentResult(w, http.StatusOK, result, err)
	}))
	mux.HandleFunc("GET /folder-statuses", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListStatuses(r.Context())
		documentResult(w, 200, result, err)
	})
	mux.HandleFunc("GET /folder-locations", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListLocations(r.Context())
		documentResult(w, 200, result, err)
	})
	for _, field := range []string{"status", "location"} {
		mux.HandleFunc("PATCH /expedientes/{expedienteID}/folders/{folderID}/"+field, documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
			id, err := strconv.ParseInt(r.PathValue("folderID"), 10, 64)
			if err != nil || id <= 0 {
				failure(w, services.ErrFolderInvalid)
				return
			}
			var result services.FolderResponse
			if field == "status" {
				var input services.UpdateFolderStatusInput
				if !decode(w, r, &input) {
					return
				}
				result, err = service.UpdateFolderStatus(r.Context(), e, id, input)
			} else {
				var input services.UpdateFolderLocationInput
				if !decode(w, r, &input) {
					return
				}
				result, err = service.UpdateFolderLocation(r.Context(), e, id, input)
			}
			documentResult(w, 200, result, err)
		}))
	}
	mux.HandleFunc("POST /folder-types", func(w http.ResponseWriter, r *http.Request) {
		var input services.CreateFolderTypeInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateFolderType(r.Context(), input)
		documentResult(w, http.StatusCreated, result, err)
	})
}
