package grpc_utils

import (
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/config"
)

func DialGRPC(address string, prometheusEndpoint string, grpcMessageSize int, tracingConfig *config.TracingConfig) (*grpc.ClientConn, error) {
	dialOpts, err := GetGRPCClientOpts(prometheusEndpoint, grpcMessageSize, tracingConfig)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
