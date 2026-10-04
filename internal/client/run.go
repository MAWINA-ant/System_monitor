package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
)

// Run requests the statistics stream and prints every received snapshot to out.
// It returns nil when the stream ends or ctx is cancelled.
func Run(ctx context.Context, c pb.StatsServiceClient, opts Options, out io.Writer) error {
	stream, err := c.GetStats(ctx, &pb.StatsRequest{NSeconds: opts.N, MSeconds: opts.M})
	if err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

	for {
		snapshot, err := stream.Recv()
		if errors.Is(err, io.EOF) || (err != nil && ctx.Err() != nil) {
			break
		}
		if err != nil {
			return fmt.Errorf("receive snapshot: %w", err)
		}

		if _, err := io.WriteString(out, Format(snapshot, opts.Sections)+"\n"); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	return nil //nolint:nilerr // io.EOF and cancellation by the user end the stream normally
}
