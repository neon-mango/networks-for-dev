package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"notes/internal/note"
	"notes/internal/storage"
)

type Handler struct {
	store *storage.Storage
}

func NewHandler(store *storage.Storage) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /notes", h.handleList)
	mux.HandleFunc("POST /notes", h.handleCreate)
	mux.HandleFunc("GET /notes/{title}", h.handleGet)
	mux.HandleFunc("PUT /notes/{title}", h.handleUpdate)
	mux.HandleFunc("DELETE /notes/{title}", h.handleDelete)
	return mux
}

type createRequest struct {
	Key   string `json:"key" xml:"key"`
	Title string `json:"title" xml:"title"`
	Value string `json:"value" xml:"value"`
}

type updateRequest struct {
	Value string `json:"value" xml:"value"`
}

type errorResponse struct {
	Error string `json:"error" xml:"error"`
}

func wantsXML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/xml")
}

func decodeBody(r *http.Request, v any) error {
	if strings.Contains(r.Header.Get("Content-Type"), "application/xml") {
		return decodeXML(r, v)
	}
	return decodeJSON(r, v)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, r, http.StatusOK, h.store.List())
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	title, err := note.TitleFromString(req.Title)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.store.Create(req.Key, title, req.Value)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	writeResponse(w, r, http.StatusCreated, n)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.store.Get(title)
	if err != nil {
		writeError(w, r, statusForErr(err), err.Error())
		return
	}
	writeResponse(w, r, http.StatusOK, n)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req updateRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	n, err := h.store.Update(title, req.Value)
	if err != nil {
		writeError(w, r, statusForErr(err), err.Error())
		return
	}
	writeResponse(w, r, http.StatusOK, n)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.Delete(title); err != nil {
		writeError(w, r, statusForErr(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func statusForErr(err error) int {
	if errors.Is(err, storage.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func writeResponse(w http.ResponseWriter, r *http.Request, status int, v any) {
	if wantsXML(r) {
		writeXML(w, status, v)
		return
	}
	writeJSON(w, status, v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	writeResponse(w, r, status, errorResponse{Error: msg})
}
