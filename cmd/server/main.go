package main

import (
	"context"
	"flag"
	"log"
	"net"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/server"
	"google.golang.org/grpc"
)

func main() {
	port := flag.String("port", "8080", "gRPC server port")
	flag.Parse()

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", ":"+*port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterStatsServiceServer(grpcServer, server.New())

	log.Printf("server listening on :%s", *port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
