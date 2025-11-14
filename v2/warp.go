package v2

import (
	"context"

	pb "github.com/hiddify/hiddify-core/hiddifyrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *CoreService) GenerateWarpConfig(ctx context.Context, in *pb.GenerateWarpConfigRequest) (*pb.WarpGenerationResponse, error) {
	return GenerateWarpConfig(in)
}

func GenerateWarpConfig(in *pb.GenerateWarpConfigRequest) (*pb.WarpGenerationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "warp functionality has been removed")
}
