package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("client stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, err := client.ParseFlags(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.NewClient(opts.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", opts.Addr, err)
	}
	defer conn.Close()

	slog.Info("requesting statistics, the first snapshot comes after m seconds",
		"addr", opts.Addr, "n", opts.N, "m", opts.M)

	return client.Run(ctx, pb.NewStatsServiceClient(conn), opts, os.Stdout)
}
