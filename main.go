package main

import (
	"log"
	"net/http"
	"os"

	"notes-backend/server"
	notesv1connect "notes-backend/proto/gen/notes/v1/notesv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"connectrpc.com/grpcreflect"
)

func main() {
	mux := http.NewServeMux()

	// Get data directory from environment variable, default to "./data"
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	path, handler := notesv1connect.NewNotesServiceHandler(
		server.NewNotesServer(dataDir),
	)
	mux.Handle(path, handler)

	// Enable gRPC reflection for tools like grpcurl
	reflector := grpcreflect.NewStaticReflector(
		"notes.v1.NotesService",
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	address := ":8080"
	log.Println("ConnectRPC Server läuft auf", address)
	
	// Enable HTTP/2 support for gRPC clients
	log.Fatal(http.ListenAndServe(address, h2c.NewHandler(mux, &http2.Server{})))
}
