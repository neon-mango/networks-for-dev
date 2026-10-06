package grpcapi

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	notespb "notes/grpc"
	"notes/internal/storage"
)

func startGRPC(t *testing.T) notespb.NotesServiceClient {
	t.Helper()
	ln := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	notespb.RegisterNotesServiceServer(srv, NewServer(storage.New()))
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return ln.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return notespb.NewNotesServiceClient(conn)
}

func TestGRPC(t *testing.T) {
	client := startGRPC(t)
	ctx := context.Background()

	n1, err := client.CreateNote(ctx, &notespb.CreateNoteRequest{Key: "k1", Title: "т1", Value: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if n1.GetKey() != "k1" || n1.GetTitle() != "т1" || n1.GetValue() != "v1" || !n1.GetUpdatedAt().IsValid() {
		t.Fatalf("create: got %+v", n1)
	}

	n2, err := client.CreateNote(ctx, &notespb.CreateNoteRequest{Key: "k2", Title: "т2", Value: "v2"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.GetNote(ctx, &notespb.GetNoteRequest{Title: "т1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetKey() != "k1" {
		t.Fatalf("get: got %+v", got)
	}

	_, err = client.GetNote(ctx, &notespb.GetNoteRequest{Title: "т9"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("get missing: got %v", status.Code(err))
	}

	updated, err := client.UpdateNote(ctx, &notespb.UpdateNoteRequest{Title: "т1", Value: "v1 обновлено"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetValue() != "v1 обновлено" {
		t.Fatalf("update: got %+v", updated)
	}

	list, err := client.ListNotes(ctx, &notespb.ListNotesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetNotes()) != 2 || list.GetNotes()[0].GetTitle() != "т1" || list.GetNotes()[1].GetTitle() != "т2" {
		t.Fatalf("list: got %+v", list.GetNotes())
	}
	if !n2.GetUpdatedAt().AsTime().After(n1.GetUpdatedAt().AsTime()) {
		t.Fatalf("timestamps: n1 %v n2 %v", n1.GetUpdatedAt(), n2.GetUpdatedAt())
	}

	_, err = client.DeleteNote(ctx, &notespb.DeleteNoteRequest{Title: "т2"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetNote(ctx, &notespb.GetNoteRequest{Title: "т2"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("get deleted: got %v", status.Code(err))
	}

	_, err = client.CreateNote(ctx, &notespb.CreateNoteRequest{Key: "k3", Title: "   ", Value: "v3"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("create empty title: got %v", status.Code(err))
	}
}
