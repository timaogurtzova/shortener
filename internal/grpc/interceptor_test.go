package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTimeoutInterceptorPreservesHandlerError(t *testing.T) {
	interceptor := timeoutUnaryServerInterceptor(10 * time.Millisecond)
	wantErr := status.Error(codes.FailedPrecondition, "business rule failed")

	response, err := interceptor(
		context.Background(),
		nil,
		nil,
		func(ctx context.Context, _ any) (any, error) {
			<-ctx.Done()
			return nil, wantErr
		},
	)

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Equal(t, status.Convert(wantErr).Message(), status.Convert(err).Message())
	assert.Nil(t, response)
}
