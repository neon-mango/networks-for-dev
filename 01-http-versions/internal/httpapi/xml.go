package httpapi

import (
	"encoding/xml"
	"net/http"

	"notes/internal/note"
)

type xmlNotes struct {
	XMLName xml.Name    `xml:"notes"`
	Notes   []note.Note `xml:"note"`
}

func decodeXML(r *http.Request, v any) error {
	return xml.NewDecoder(r.Body).Decode(v)
}

func writeXML(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	if list, ok := v.([]note.Note); ok {
		_ = xml.NewEncoder(w).Encode(xmlNotes{Notes: list})
		return
	}
	_ = xml.NewEncoder(w).Encode(v)
}
