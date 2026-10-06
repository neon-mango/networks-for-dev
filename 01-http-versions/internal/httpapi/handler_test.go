package httpapi

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notes/internal/note"
	"notes/internal/storage"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewHandler(storage.New()).Routes())
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path, body, accept, contentType string) *http.Response {
	t.Helper()
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, srv.URL+path, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCRUDJSON(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodPost, "/notes", `{"key":"k1","title":"t1","value":"v1"}`, "", "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status %d", resp.StatusCode)
	}
	var n1 note.Note
	if err := json.NewDecoder(resp.Body).Decode(&n1); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n1.Title != "t1" || n1.Key != "k1" {
		t.Fatalf("create: got %+v", n1)
	}

	resp = do(t, srv, http.MethodPost, "/notes", `{"key":"k2","title":"t2","value":"v2"}`, "", "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create t2: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes", "", "", "")
	var list []note.Note
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list) != 2 || list[0].Title != "t2" || list[1].Title != "t1" {
		t.Fatalf("list order: got %+v", list)
	}

	resp = do(t, srv, http.MethodPut, "/notes/t1", `{"value":"v1 updated"}`, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes", "", "", "")
	list = nil
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list) != 2 || list[0].Title != "t1" || list[0].Value != "v1 updated" {
		t.Fatalf("list order after update: got %+v", list)
	}

	resp = do(t, srv, http.MethodDelete, "/notes/t2", "", "", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes/t2", "", "", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted: status %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCRUDXML(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodPost, "/notes",
		`<createRequest><key>k1</key><title>т1</title><value>v1</value></createRequest>`,
		"application/xml", "application/xml")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("create: content-type %q", ct)
	}
	var n1 note.Note
	if err := xml.NewDecoder(resp.Body).Decode(&n1); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n1.Title != "т1" || n1.Key != "k1" {
		t.Fatalf("create: got %+v", n1)
	}

	resp = do(t, srv, http.MethodPost, "/notes",
		`<createRequest><key>k2</key><title>т2</title><value>v2</value></createRequest>`,
		"application/xml", "application/xml")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create т2: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes", "", "application/xml", "")
	var list xmlNotes
	if err := xml.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list.Notes) != 2 || list.Notes[0].Title != "т2" || list.Notes[1].Title != "т1" {
		t.Fatalf("list order: got %+v", list.Notes)
	}

	resp = do(t, srv, http.MethodPut, "/notes/т1",
		`<updateRequest><value>v1 обновлено</value></updateRequest>`,
		"application/xml", "application/xml")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes", "", "application/xml", "")
	list = xmlNotes{}
	if err := xml.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list.Notes) != 2 || list.Notes[0].Title != "т1" || list.Notes[0].Value != "v1 обновлено" {
		t.Fatalf("list order after update: got %+v", list.Notes)
	}

	resp = do(t, srv, http.MethodDelete, "/notes/т2", "", "application/xml", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, srv, http.MethodGet, "/notes/т2", "", "application/xml", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted: status %d", resp.StatusCode)
	}
	var errResp errorResponse
	if err := xml.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if errResp.Error != "note not found" {
		t.Fatalf("error body: got %+v", errResp)
	}
}
