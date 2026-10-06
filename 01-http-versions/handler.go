package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

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
	Key   string `json:"key"`
	Title string `json:"title"`
	Value string `json:"value"`
}

type updateRequest struct {
	Value string `json:"value"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.List())
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	title, err := note.TitleFromString(req.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.store.Create(req.Key, title, req.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.store.Get(title)
	if err != nil {
		writeError(w, statusForErr(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	n, err := h.store.Update(title, req.Value)
	if err != nil {
		writeError(w, statusForErr(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	title, err := note.TitleFromString(r.PathValue("title"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.Delete(title); err != nil {
		writeError(w, statusForErr(err), err.Error())
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
