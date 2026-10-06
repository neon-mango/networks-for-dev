package httpapi

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/quic-go/quic-go/http3"

	"notes/internal/cert"
	"notes/internal/storage"
)

func TestHTTP3(t *testing.T) {
	c, err := cert.SelfSigned()
	if err != nil {
		t.Fatal(err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	srv := &http3.Server{
		Handler:   NewHandler(storage.New()).Routes(),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{c}},
	}
	go srv.Serve(conn)

	tr := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer tr.Close()
	client := &http.Client{Transport: tr}

	base := "https://" + conn.LocalAddr().String()

	resp, err := client.Post(base+"/notes", "application/json", strings.NewReader(`{"key":"k1","title":"т1","value":"v1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status %d", resp.StatusCode)
	}
	if resp.Proto != "HTTP/3.0" {
		t.Fatalf("create: proto %q", resp.Proto)
	}
	resp.Body.Close()

	resp, err = client.Get(base + "/notes")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Proto != "HTTP/3.0" {
		t.Fatalf("list: proto %q", resp.Proto)
	}
	resp.Body.Close()
}
