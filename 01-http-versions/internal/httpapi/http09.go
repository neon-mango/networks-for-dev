package httpapi

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"notes/internal/note"
)

func (h *Handler) ServeHTTP09(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go h.handleHTTP09Conn(conn)
	}
}

func (h *Handler) handleHTTP09Conn(conn net.Conn) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	body, err := h.handleHTTP09(strings.TrimSpace(line))
	if err != nil {
		fmt.Fprintln(conn, "error: "+err.Error())
		return
	}
	fmt.Fprint(conn, body)
}

func (h *Handler) handleHTTP09(path string) (string, error) {
	if path == "/notes" {
		var sb strings.Builder
		for _, n := range h.store.List() {
			fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\n", n.Key, note.TitleToString(n.Title), n.Value, n.UpdatedAt.Format(time.RFC3339))
		}
		return sb.String(), nil
	}
	if strings.HasPrefix(path, "/notes/") {
		title, err := note.TitleFromString(strings.TrimPrefix(path, "/notes/"))
		if err != nil {
			return "", err
		}
		n, err := h.store.Get(title)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s\t%s\t%s\t%s\n", n.Key, note.TitleToString(n.Title), n.Value, n.UpdatedAt.Format(time.RFC3339)), nil
	}
	return "", errors.New("unknown path")
}
