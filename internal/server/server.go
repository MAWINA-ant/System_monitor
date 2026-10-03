package server

import (
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
)

type Server struct {
	pb.UnimplementedStatsServiceServer
}

func New() *Server {
	return &Server{}
}

func (s *Server) GetStats(req *pb.StatsRequest, stream pb.StatsService_GetStatsServer) error {
	ticker := time.NewTicker(time.Duration(req.GetNSeconds()) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			if err := stream.Send(&pb.Snapshot{}); err != nil {
				return err
			}
		}
	}
}
