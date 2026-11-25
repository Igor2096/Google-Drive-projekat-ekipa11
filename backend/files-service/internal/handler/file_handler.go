package handler

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"files-service/internal/middleware"
	"files-service/internal/service"
)

// FileHandler - otpremanje datoteka (zahtev 1.5, task T-23).
type FileHandler struct {
	svc           *service.FileService
	maxUploadSize int64
}

func NewFileHandler(svc *service.FileService, maxUploadSize int64) *FileHandler {
	return &FileHandler{svc: svc, maxUploadSize: maxUploadSize}
}

func (h *FileHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /files", h.upload)
}

// Koliko multipart zahteva sme da stane u memoriju; ostatak Go sam sprema
// u privremeni fajl na disku.
const multipartMemory = 8 << 20 // 8 MB

// POST /files   (multipart/form-data)
//
//	file     - sama datoteka (obavezno)
//	folderId - ID foldera; prazno ili izostavljeno znaci koren
func (h *FileHandler) upload(w http.ResponseWriter, r *http.Request) {
	// Gornja granica velicine zahteva; bez ovoga bi jedan veliki upload
	// mogao da popuni disk servera.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "datoteka je prevelika")
			return
		}
		writeError(w, http.StatusBadRequest, "zahtev mora biti multipart/form-data sa poljem \"file\"")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll() // brise privremene fajlove
		}
	}()

	content, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "nedostaje polje \"file\"")
		return
	}
	defer content.Close()

	name := baseName(header.Filename)
	file, err := h.svc.Upload(r.Context(), middleware.UserID(r), service.UploadInput{
		Name:     name,
		FolderID: strings.TrimSpace(r.FormValue("folderId")),
		MimeType: contentType(header.Header.Get("Content-Type"), name),
		Size:     header.Size,
		Content:  content,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, file)
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

// contentType uzima tip koji je poslao klijent; ako ga nema ili je uopsten,
// pokusava da ga pogodi po ekstenziji.
func contentType(declared, name string) string {
	declared = strings.TrimSpace(declared)
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	if byExt := mime.TypeByExtension(filepath.Ext(name)); byExt != "" {
		return byExt
	}
	return declared
}
