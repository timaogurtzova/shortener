package grpcserver

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/config"
	shortenerpb "github.com/timaogurtzova/shortener/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type blockingShortenerServer struct {
	shortenerpb.UnimplementedShortenerServiceServer
	started chan struct{}
	release chan struct{}
}

func (s *blockingShortenerServer) ShortenURL(
	context.Context,
	*shortenerpb.URLShortenRequest,
) (*shortenerpb.URLShortenResponse, error) {
	close(s.started)
	<-s.release
	return &shortenerpb.URLShortenResponse{Result: "completed"}, nil
}

func TestGRPCServerRunContextWaitsForActiveRequest(t *testing.T) {
	listener := newTCPListener(t)
	address := listener.Addr().String()
	handler := &blockingShortenerServer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server, err := NewServer(&config.Configuration{
		Server: config.ServerConfiguration{IdleTimeout: time.Second},
		GRPC:   config.GRPCConfiguration{Address: address},
	}, handler)
	require.NoError(t, err)

	runContext, cancelRun := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- server.runWithListener(runContext, listener)
	}()

	connection, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = connection.Close()
		cancelRun()
		select {
		case <-handler.release:
		default:
			close(handler.release)
		}
	})

	responseResult := make(chan *shortenerpb.URLShortenResponse, 1)
	responseError := make(chan error, 1)
	go func() {
		response, callErr := shortenerpb.NewShortenerServiceClient(connection).ShortenURL(
			context.Background(),
			&shortenerpb.URLShortenRequest{Url: "https://example.com"},
		)
		if callErr != nil {
			responseError <- callErr
			return
		}
		responseResult <- response
	}()

	select {
	case <-handler.started:
	case <-time.After(time.Second):
		t.Fatal("request did not reach gRPC handler")
	}
	cancelRun()

	select {
	case runErr := <-runResult:
		t.Fatalf("server returned before active request completed: %v", runErr)
	case <-time.After(50 * time.Millisecond):
	}

	close(handler.release)
	select {
	case callErr := <-responseError:
		t.Fatalf("gRPC request failed during graceful shutdown: %v", callErr)
	case response := <-responseResult:
		assert.Equal(t, "completed", response.GetResult())
	case <-time.After(time.Second):
		t.Fatal("active gRPC request was not completed")
	}

	select {
	case runErr := <-runResult:
		require.NoError(t, runErr)
	case <-time.After(time.Second):
		t.Fatal("gRPC server did not stop")
	}
}

func TestGRPCServerUsesTLSWhenHTTPSEnabled(t *testing.T) {
	listener := newTCPListener(t)
	address := listener.Addr().String()
	handler := &blockingShortenerServer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	close(handler.release)

	server, err := NewServer(&config.Configuration{
		Server: config.ServerConfiguration{
			EnableHTTPS: true,
			IdleTimeout: time.Second,
		},
		GRPC: config.GRPCConfiguration{Address: address},
	}, handler)
	require.NoError(t, err)

	runContext, cancelRun := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- server.runWithListener(runContext, listener)
	}()

	connection, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // Самоподписанный сертификат локального тестового сервера.
		})),
	)
	require.NoError(t, err)

	response, err := shortenerpb.NewShortenerServiceClient(connection).ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com"},
	)
	require.NoError(t, err)
	assert.Equal(t, "completed", response.GetResult())

	require.NoError(t, connection.Close())
	cancelRun()
	select {
	case runErr := <-runResult:
		require.NoError(t, runErr)
	case <-time.After(time.Second):
		t.Fatal("TLS gRPC server did not stop")
	}
}

type panicShortenerServer struct {
	shortenerpb.UnimplementedShortenerServiceServer
}

func (*panicShortenerServer) ShortenURL(
	context.Context,
	*shortenerpb.URLShortenRequest,
) (*shortenerpb.URLShortenResponse, error) {
	panic("unexpected handler panic")
}

func TestGRPCServerRecoversHandlerPanic(t *testing.T) {
	listener := newTCPListener(t)
	server, err := NewServer(&config.Configuration{
		Server: config.ServerConfiguration{
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		GRPC: config.GRPCConfiguration{Address: listener.Addr().String()},
	}, &panicShortenerServer{})
	require.NoError(t, err)

	runContext, cancelRun := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- server.runWithListener(runContext, listener)
	}()

	connection, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	_, err = shortenerpb.NewShortenerServiceClient(connection).ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com"},
	)
	assert.Equal(t, codes.Internal, status.Code(err))

	require.NoError(t, connection.Close())
	cancelRun()
	select {
	case runErr := <-runResult:
		require.NoError(t, runErr)
	case <-time.After(time.Second):
		t.Fatal("gRPC server did not stop after recovered panic")
	}
}

type deadlineAwareShortenerServer struct {
	shortenerpb.UnimplementedShortenerServiceServer
}

func (*deadlineAwareShortenerServer) ShortenURL(
	ctx context.Context,
	_ *shortenerpb.URLShortenRequest,
) (*shortenerpb.URLShortenResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestGRPCServerAppliesWriteTimeoutToUnaryRPC(t *testing.T) {
	const requestTimeout = 25 * time.Millisecond

	listener := newTCPListener(t)
	server, err := NewServer(&config.Configuration{
		Server: config.ServerConfiguration{
			ReadTimeout:  time.Second,
			WriteTimeout: requestTimeout,
		},
		GRPC: config.GRPCConfiguration{Address: listener.Addr().String()},
	}, &deadlineAwareShortenerServer{})
	require.NoError(t, err)

	runContext, cancelRun := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- server.runWithListener(runContext, listener)
	}()

	connection, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	startedAt := time.Now()
	_, err = shortenerpb.NewShortenerServiceClient(connection).ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com"},
	)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	assert.Less(t, time.Since(startedAt), time.Second)

	require.NoError(t, connection.Close())
	cancelRun()
	select {
	case runErr := <-runResult:
		require.NoError(t, runErr)
	case <-time.After(time.Second):
		t.Fatal("gRPC server did not stop after request timeout")
	}
}

func newTCPListener(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return listener
}
