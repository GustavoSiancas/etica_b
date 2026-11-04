package httpapi

import (
	"etica_back/internal/services"
	"net/http"
)

// Catalogue creation is intentionally open until administrator authentication is added.
func registerFolderTypes(mux *http.ServeMux, service DocumentAPI) {
	mux.HandleFunc("POST /folder-types", func(w http.ResponseWriter, r *http.Request) {
		var input services.CreateFolderTypeInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateFolderType(r.Context(), input)
		documentResult(w, http.StatusCreated, result, err)
	})
}
