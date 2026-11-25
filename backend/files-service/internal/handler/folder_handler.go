// Package handler - HTTP sloj: parsiranje zahteva i mapiranje gresaka na status kodove.
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"files-service/internal/middleware"
	"files-service/internal/service"
)

type FolderHandler struct {
	svc *service.FolderService
}

func NewFolderHandler(svc *service.FolderService) *FolderHandler {
	return &FolderHandler{svc: svc}
}

// Register povezuje rute (Go 1.22+ ServeMux podrzava metod i {param} u putanji).
func (h *FolderHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /folders", h.create)
	mux.HandleFunc("GET /folders/root/contents", h.rootContents)
	mux.HandleFunc("GET /folders/{id}/contents", h.contents)
	mux.HandleFunc("PATCH /folders/{id}", h.rename)
}

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"` // prazno ili izostavljeno => koren
}

type renameFolderRequest struct {
	Name string `json:"name"`
}

// POST /folders  {"name": "Skola", "parentId": "<id ili prazno>"}
func (h *FolderHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	folder, err := h.svc.CreateFolder(r.Context(), middleware.UserID(r), req.Name, req.ParentID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

// GET /folders/root/contents
func (h *FolderHandler) rootContents(w http.ResponseWriter, r *http.Request) {
	h.writeContents(w, r, "")
}

// GET /folders/{id}/contents
func (h *FolderHandler) contents(w http.ResponseWriter, r *http.Request) {
	h.writeContents(w, r, r.PathValue("id"))
}

func (h *FolderHandler) writeContents(w http.ResponseWriter, r *http.Request, folderID string) {
	contents, err := h.svc.GetContents(r.Context(), middleware.UserID(r), folderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contents)
}

// PATCH /folders/{id}  {"name": "Novi naziv"}
func (h *FolderHandler) rename(w http.ResponseWriter, r *http.Request) {
	var req renameFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	folder, err := h.svc.RenameFolder(r.Context(), middleware.UserID(r), r.PathValue("id"), req.Name)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

// ---- pomocne funkcije ----

const maxBodyBytes = 1 << 20 // 1 MB je i previse za JSON sa nazivom foldera

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "neispravan JSON u telu zahteva")
		return false
	}
	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidName), errors.Is(err, service.ErrInvalidID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound), errors.Is(err, service.ErrFileNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrDuplicate), errors.Is(err, service.ErrFileDuplicate):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "interna greska servera")
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
