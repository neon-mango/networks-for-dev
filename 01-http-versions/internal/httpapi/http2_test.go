package httpapi

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"notes/internal/storage"
)

func TestHTTP2(t *testing.T) {
	srv := httptest.NewUnstartedServer(h2c.NewHandler(NewHandler(storage.New()).Routes(), &http2.Server{}))
	srv.Start()
	defer srv.Close()

	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	resp, err := client.Post(srv.URL+"/notes", "application/json", strings.NewReader(`{"key":"k1","title":"т1","value":"v1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status %d", resp.StatusCode)
	}
	if resp.Proto != "HTTP/2.0" {
		t.Fatalf("create: proto %q", resp.Proto)
	}
	resp.Body.Close()

	resp, err = client.Get(srv.URL + "/notes")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Proto != "HTTP/2.0" {
		t.Fatalf("list: proto %q", resp.Proto)
	}
	resp.Body.Close()
}
