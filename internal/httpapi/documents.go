package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"etica_back/internal/services"
)

type DocumentAPI interface {
	ListLocations(context.Context) ([]services.FolderLocationResponse, error)
	ListStatuses(context.Context) ([]services.FolderStatusResponse, error)
	UpdateFolderStatus(context.Context, int64, int64, services.UpdateFolderStatusInput) (services.FolderResponse, error)
	UpdateFolderLocation(context.Context, int64, int64, services.UpdateFolderLocationInput) (services.FolderResponse, error)
	CreateFolderType(context.Context, services.CreateFolderTypeInput) (services.FolderTypeResponse, error)
	ListFolderTypes(context.Context) ([]services.FolderTypeResponse, error)
	ListFolders(context.Context, int64) ([]services.FolderResponse, error)
	CreateFolder(context.Context, int64, services.CreateFolderInput) (services.FolderResponse, error)
	ListFolderSections(context.Context, int64, int64) ([]services.SectionResponse, error)
	ListTypes(context.Context) ([]services.DocumentTypeEntry, error)
	Create(context.Context, services.CreateDocumentInput) (services.DocumentResponse, error)
	List(context.Context, int, int) (services.DocumentPage, error)
	Get(context.Context, int64) (services.DocumentResponse, error)
	Update(context.Context, int64, services.UpdateDocumentInput) (services.DocumentResponse, error)
	Delete(context.Context, int64) error
	ListByExpediente(context.Context, int64) ([]services.DocumentResponse, error)
	CreateSection(context.Context, int64, services.CreateSectionInput) (services.SectionResponse, error)
	ListSections(context.Context, int64) ([]services.SectionResponse, error)
	GetSection(context.Context, int64, int64) (services.SectionResponse, error)
	UpdateSection(context.Context, int64, int64, services.UpdateSectionInput) (services.SectionResponse, error)
	DeleteSection(context.Context, int64, int64) error
	CreateInSection(context.Context, int64, int64, services.CreateDocumentInput) (services.DocumentResponse, error)
	ListSectionDocuments(context.Context, int64, int64) ([]services.DocumentResponse, error)
	AttachDocument(context.Context, int64, int64, int64) error
	DetachDocument(context.Context, int64, int64, int64) error
}

// IDs are validated before any service call. Paths use distinct names for each entity.
func documentRoute(fn func(http.ResponseWriter, *http.Request, int64, int64, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := [3]int64{}
		for i, key := range []string{"expedienteID", "sectionID", "documentID"} {
			if value := r.PathValue(key); value != "" {
				id, err := strconv.ParseInt(value, 10, 64)
				if err != nil || id <= 0 {
					failure(w, services.ErrDocumentInvalid)
					return
				}
				ids[i] = id
			}
		}
		fn(w, r, ids[0], ids[1], ids[2])
	}
}
func documentResult(w http.ResponseWriter, status int, result any, err error) {
	if err != nil {
		failure(w, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	respond(w, status, result)
}
func documentPagination(r *http.Request) (int, int, bool) {
	values := [2]int{20, 0}
	for i, key := range []string{"limit", "offset"} {
		if raw, ok := r.URL.Query()[key]; ok {
			if len(raw) != 1 {
				return 0, 0, false
			}
			value, err := strconv.Atoi(raw[0])
			if err != nil {
				return 0, 0, false
			}
			values[i] = value
		}
	}
	return values[0], values[1], values[0] >= 1 && values[0] <= 100 && values[1] >= 0
}
func registerDocuments(mux *http.ServeMux, service DocumentAPI) {
	mux.HandleFunc("GET /folder-types", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListFolderTypes(r.Context())
		documentResult(w, 200, result, err)
	})
	mux.HandleFunc("GET /expedientes/{expedienteID}/folders", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		result, err := service.ListFolders(r.Context(), e)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("POST /expedientes/{expedienteID}/folders", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		var input services.CreateFolderInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateFolder(r.Context(), e, input)
		documentResult(w, 201, result, err)
	}))
	mux.HandleFunc("GET /expedientes/{expedienteID}/folders/{folderID}/sections", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		folderID, err := strconv.ParseInt(r.PathValue("folderID"), 10, 64)
		if err != nil || folderID <= 0 {
			failure(w, services.ErrSectionInvalid)
			return
		}
		result, err := service.ListFolderSections(r.Context(), e, folderID)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("GET /document-types", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListTypes(r.Context())
		documentResult(w, 200, result, err)
	})
	mux.HandleFunc("POST /documents", func(w http.ResponseWriter, r *http.Request) {
		var input services.CreateDocumentInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.Create(r.Context(), input)
		documentResult(w, 201, result, err)
	})
	mux.HandleFunc("GET /documents", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := documentPagination(r)
		if !ok {
			failure(w, services.ErrDocumentInvalid)
			return
		}
		result, err := service.List(r.Context(), limit, offset)
		documentResult(w, 200, result, err)
	})
	mux.HandleFunc("GET /documents/{documentID}", documentRoute(func(w http.ResponseWriter, r *http.Request, _, _, id int64) {
		result, err := service.Get(r.Context(), id)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("PATCH /documents/{documentID}", documentRoute(func(w http.ResponseWriter, r *http.Request, _, _, id int64) {
		var input services.UpdateDocumentInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.Update(r.Context(), id, input)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("DELETE /documents/{documentID}", documentRoute(func(w http.ResponseWriter, r *http.Request, _, _, id int64) {
		documentResult(w, 204, nil, service.Delete(r.Context(), id))
	}))
	mux.HandleFunc("GET /expedientes/{expedienteID}/documents", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		result, err := service.ListByExpediente(r.Context(), e)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("POST /expedientes/{expedienteID}/sections", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		var input services.CreateSectionInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateSection(r.Context(), e, input)
		documentResult(w, 201, result, err)
	}))
	mux.HandleFunc("GET /expedientes/{expedienteID}/sections", documentRoute(func(w http.ResponseWriter, r *http.Request, e, _, _ int64) {
		result, err := service.ListSections(r.Context(), e)
		documentResult(w, 200, result, err)
	}))
	const sectionPath = "/expedientes/{expedienteID}/sections/{sectionID}"
	mux.HandleFunc("GET "+sectionPath, documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, _ int64) {
		result, err := service.GetSection(r.Context(), e, s)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("PATCH "+sectionPath, documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, _ int64) {
		var input services.UpdateSectionInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.UpdateSection(r.Context(), e, s, input)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("DELETE "+sectionPath, documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, _ int64) {
		documentResult(w, 204, nil, service.DeleteSection(r.Context(), e, s))
	}))
	mux.HandleFunc("POST "+sectionPath+"/documents", documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, _ int64) {
		var input services.CreateDocumentInput
		if !decode(w, r, &input) {
			return
		}
		result, err := service.CreateInSection(r.Context(), e, s, input)
		documentResult(w, 201, result, err)
	}))
	mux.HandleFunc("GET "+sectionPath+"/documents", documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, _ int64) {
		result, err := service.ListSectionDocuments(r.Context(), e, s)
		documentResult(w, 200, result, err)
	}))
	mux.HandleFunc("POST "+sectionPath+"/documents/{documentID}", documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, d int64) {
		documentResult(w, 204, nil, service.AttachDocument(r.Context(), e, s, d))
	}))
	mux.HandleFunc("DELETE "+sectionPath+"/documents/{documentID}", documentRoute(func(w http.ResponseWriter, r *http.Request, e, s, d int64) {
		documentResult(w, 204, nil, service.DetachDocument(r.Context(), e, s, d))
	}))
}
