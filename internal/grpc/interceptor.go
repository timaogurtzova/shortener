package grpcserver

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func recoveryUnaryServerInterceptor(
	ctx context.Context,
	request any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (response any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().
				Interface("panic", recovered).
				Str("method", info.FullMethod).
				Str("stack", string(debug.Stack())).
				Msg("recovered from panic in gRPC handler")
			response = nil
			err = status.Error(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, request)
}

func timeoutUnaryServerInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if timeout <= 0 {
			return handler(ctx, request)
		}

		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		response, err := handler(requestCtx, request)
		if err != nil {
			return response, err
		}
		if requestErr := requestCtx.Err(); requestErr != nil {
			return nil, status.FromContextError(requestErr).Err()
		}

		return response, nil
	}
}
