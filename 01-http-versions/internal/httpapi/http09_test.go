package httpapi

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"notes/internal/storage"
)

func startHTTP09(t *testing.T) (net.Listener, *storage.Storage) {
	t.Helper()
	store := storage.New()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go NewHandler(store).ServeHTTP09(ln)
	return ln, store
}

func query09(t *testing.T, addr, line string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(line + "\n")); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHTTP09(t *testing.T) {
	ln, store := startHTTP09(t)
	addr := ln.Addr().String()

	if _, err := store.Create("k1", "т1", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create("k2", "т2", "v2"); err != nil {
		t.Fatal(err)
	}

	resp := query09(t, addr, "/notes")
	lines := strings.Split(strings.TrimSpace(resp), "\n")
	if len(lines) != 2 {
		t.Fatalf("list: got %q", resp)
	}
	fields := strings.Split(lines[0], "\t")
	if fields[0] != "k2" || fields[1] != "т2" {
		t.Fatalf("list order: got %q", lines[0])
	}

	resp = query09(t, addr, "/notes/т1")
	fields = strings.Split(strings.TrimSpace(resp), "\t")
	if len(fields) != 4 || fields[0] != "k1" || fields[1] != "т1" || fields[2] != "v1" {
		t.Fatalf("get: got %q", resp)
	}

	resp = query09(t, addr, "/bad")
	if resp != "error: unknown path\n" {
		t.Fatalf("unknown path: got %q", resp)
	}

	resp = query09(t, addr, "/notes/т9")
	if resp != "error: note not found\n" {
		t.Fatalf("not found: got %q", resp)
	}
}
