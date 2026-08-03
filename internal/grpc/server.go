package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
	"github.com/timaogurtzova/shortener/internal/service"
	"github.com/timaogurtzova/shortener/internal/tlsconfig"
	shortenerpb "github.com/timaogurtzova/shortener/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const serverShutdownTimeout = 10 * time.Second

const maxGRPCRequestSize = service.MaxOriginalURLLength + 1024

// Server управляет жизненным циклом gRPC-сервера.
type Server struct {
	address    string
	grpcServer *grpc.Server
}

// NewServer создаёт и настраивает gRPC-сервер.
func NewServer(cfg *config.Configuration, handler shortenerpb.ShortenerServiceServer) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("configuration is nil")
	}
	if handler == nil {
		return nil, errors.New("gRPC handler is nil")
	}

	options := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxGRPCRequestSize),
		grpc.ChainUnaryInterceptor(
			recoveryUnaryServerInterceptor,
			timeoutUnaryServerInterceptor(cfg.Server.WriteTimeout),
		),
	}
	if cfg.Server.IdleTimeout > 0 {
		options = append(options, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: cfg.Server.IdleTimeout,
		}))
	}
	if cfg.Server.ReadTimeout > 0 {
		options = append(options, grpc.ConnectionTimeout(cfg.Server.ReadTimeout))
	}
	if cfg.Server.EnableHTTPS {
		tlsConfig, err := tlsconfig.New(cfg.GRPC.Address)
		if err != nil {
			return nil, fmt.Errorf("create gRPC TLS config: %w", err)
		}
		options = append(options, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	grpcServer := grpc.NewServer(options...)
	shortenerpb.RegisterShortenerServiceServer(grpcServer, handler)

	return &Server{
		address:    cfg.GRPC.Address,
		grpcServer: grpcServer,
	}, nil
}

// Run запускает gRPC-сервер и завершает его по сигналу ОС.
func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	return s.RunContext(ctx)
}

// RunContext запускает gRPC-сервер и корректно завершает его после отмены контекста.
func (s *Server) RunContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address, err)
	}

	return s.runWithListener(ctx, listener)
}

func (s *Server) runWithListener(ctx context.Context, listener net.Listener) error {
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", listener.Addr().String()).Msg("gRPC server started")
		errCh <- s.grpcServer.Serve(listener)
	}()

	select {
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			return fmt.Errorf("serve gRPC: %w", serveErr)
		}
		return nil
	case <-ctx.Done():
		log.Info().Msg("gRPC shutdown requested")
	}

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	timer := time.NewTimer(serverShutdownTimeout)
	defer timer.Stop()

	select {
	case <-stopped:
	case <-timer.C:
		log.Warn().Msg("gRPC graceful shutdown timed out")
		s.grpcServer.Stop()
		<-stopped
	}

	serveErr := <-errCh
	if serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
		return fmt.Errorf("serve gRPC during shutdown: %w", serveErr)
	}

	log.Info().Msg("gRPC server shutdown gracefully")
	return nil
}
