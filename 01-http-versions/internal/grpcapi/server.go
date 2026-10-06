package grpcapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	notespb "notes/grpc"
	"notes/internal/note"
	"notes/internal/storage"
)

type Server struct {
	notespb.UnimplementedNotesServiceServer
	store *storage.Storage
}

func NewServer(store *storage.Storage) *Server {
	return &Server{store: store}
}

func toProto(n note.Note) *notespb.Note {
	return &notespb.Note{
		Key:       n.Key,
		Title:     note.TitleToString(n.Title),
		Value:     n.Value,
		UpdatedAt: timestamppb.New(n.UpdatedAt),
	}
}

func (s *Server) CreateNote(ctx context.Context, req *notespb.CreateNoteRequest) (*notespb.Note, error) {
	title, err := note.TitleFromString(req.GetTitle())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	n, err := s.store.Create(req.GetKey(), title, req.GetValue())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return toProto(n), nil
}

func (s *Server) GetNote(ctx context.Context, req *notespb.GetNoteRequest) (*notespb.Note, error) {
	title, err := note.TitleFromString(req.GetTitle())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	n, err := s.store.Get(title)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return toProto(n), nil
}

func (s *Server) UpdateNote(ctx context.Context, req *notespb.UpdateNoteRequest) (*notespb.Note, error) {
	title, err := note.TitleFromString(req.GetTitle())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	n, err := s.store.Update(title, req.GetValue())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return toProto(n), nil
}

func (s *Server) DeleteNote(ctx context.Context, req *notespb.DeleteNoteRequest) (*emptypb.Empty, error) {
	title, err := note.TitleFromString(req.GetTitle())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.store.Delete(title); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListNotes(ctx context.Context, req *notespb.ListNotesRequest) (*notespb.ListNotesResponse, error) {
	notes := s.store.List()
	out := make([]*notespb.Note, 0, len(notes))
	for _, n := range notes {
		out = append(out, toProto(n))
	}
	return &notespb.ListNotesResponse{Notes: out}, nil
}
