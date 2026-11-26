package handler

import (
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"files-service/internal/middleware"
	"files-service/internal/service"
)

// FileHandler - HTTP sloj za datoteke.
type FileHandler struct {
	svc           *service.FileService
	maxUploadSize int64
}

func NewFileHandler(svc *service.FileService, maxUploadSize int64) *FileHandler {
	return &FileHandler{svc: svc, maxUploadSize: maxUploadSize}
}

func (h *FileHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /files", h.upload)
	mux.HandleFunc("PUT /files/{id}", h.uploadNewVersion)
	mux.HandleFunc("PATCH /files/{id}", h.rename)
	mux.HandleFunc("GET /files/{id}/verify", h.verify)
}

// Koliko multipart zahteva sme da stane u memoriju; ostatak Go sam sprema
// u privremeni fajl na disku.
const multipartMemory = 8 << 20 // 8 MB

// POST /files   (multipart/form-data)
//
//	file     - sama datoteka (obavezno)
//	folderId - ID foldera; prazno ili izostavljeno znaci koren
func (h *FileHandler) upload(w http.ResponseWriter, r *http.Request) {
	content, header, ok := h.readFilePart(w, r)
	if !ok {
		return
	}
	defer content.Close()
	defer cleanupMultipart(r)

	file, err := h.svc.Upload(r.Context(), middleware.UserID(r), service.UploadInput{
		Name:     baseName(header.Filename),
		FolderID: strings.TrimSpace(r.FormValue("folderId")),
		Size:     header.Size,
		Content:  content,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, file)
}

// PUT /files/{id}   (multipart/form-data, polje "file")
//
// Naziv i folder se ne menjaju; menja se sadrzaj, velicina, tip i broj verzije.
func (h *FileHandler) uploadNewVersion(w http.ResponseWriter, r *http.Request) {
	content, header, ok := h.readFilePart(w, r)
	if !ok {
		return
	}
	defer content.Close()
	defer cleanupMultipart(r)

	file, err := h.svc.UploadNewVersion(r.Context(), middleware.UserID(r), r.PathValue("id"), service.NewVersionInput{
		Size:    header.Size,
		Content: content,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

type renameFileRequest struct {
	Name string `json:"name"`
}

// PATCH /files/{id}  {"name": "novi naziv.pdf"}  (zahtev 1.6)
func (h *FileHandler) rename(w http.ResponseWriter, r *http.Request) {
	var req renameFileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	file, err := h.svc.RenameFile(r.Context(), middleware.UserID(r), r.PathValue("id"), req.Name)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

// GET /files/{id}/verify - provera integriteta (2.18)
func (h *FileHandler) verify(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.VerifyIntegrity(r.Context(), middleware.UserID(r), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// readFilePart je zajednicki deo oba zahteva sa datotekom: ogranici velicinu,
// raspakuje multipart i izvuce polje "file". Vraca ok=false ako je odgovor
// vec poslat.
func (h *FileHandler) readFilePart(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, bool) {
	// Gornja granica velicine zahteva; bez ovoga bi jedan veliki upload
	// mogao da popuni disk servera.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "datoteka je prevelika")
			return nil, nil, false
		}
		writeError(w, http.StatusBadRequest, "zahtev mora biti multipart/form-data sa poljem \"file\"")
		return nil, nil, false
	}

	content, header, err := r.FormFile("file")
	if err != nil {
		cleanupMultipart(r)
		writeError(w, http.StatusBadRequest, "nedostaje polje \"file\"")
		return nil, nil, false
	}
	return content, header, true
}

// cleanupMultipart brise privremene fajlove koje je Go napravio za velike delove.
func cleanupMultipart(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

// baseName odbacuje putanju iz naziva koji je poslao browser.
// Neki klijenti salju ceo "C:\Users\...\slika.png", pa se gleda i \ i /.
func baseName(filename string) string {
	filename = filepath.Base(filename)
	if i := strings.LastIndexAny(filename, `/\`); i >= 0 {
		filename = filename[i+1:]
	}
	return strings.TrimSpace(filename)
}
