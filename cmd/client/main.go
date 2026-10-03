package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "gRPC server address")
	flag.Parse()

	if err := run(*addr); err != nil {
		log.Fatal(err)
	}
}

func run(addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := pb.NewStatsServiceClient(conn)

	stream, err := client.GetStats(context.Background(), &pb.StatsRequest{NSeconds: 5, MSeconds: 15})
	if err != nil {
		return fmt.Errorf("failed to call GetStats: %w", err)
	}

	for {
		snapshot, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}
		log.Printf("received snapshot: %+v", snapshot)
	}
}
