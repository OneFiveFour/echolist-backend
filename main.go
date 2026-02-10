package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "notes-backend/gen/notes"
	"notes-backend/server"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNotesServiceServer(
		grpcServer,
		server.NewNotesServer("./data"),
	)
	reflection.Register(grpcServer)

	log.Println("gRPC Server läuft auf :50051")
	log.Fatal(grpcServer.Serve(lis))
}
