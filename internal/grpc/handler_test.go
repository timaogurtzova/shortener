package grpcserver_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/auth"
	grpcserver "github.com/timaogurtzova/shortener/internal/grpc"
	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/service"
	shortenerpb "github.com/timaogurtzova/shortener/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufconnSize = 1024 * 1024

type mockShortener struct {
	createFunc       func(context.Context, string, string) (string, error)
	resolveFunc      func(context.Context, string) (string, error)
	findByUserIDFunc func(context.Context, string) ([]model.UserURL, error)
}

func (m *mockShortener) Create(ctx context.Context, url, userID string) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, url, userID)
	}
	return "", errors.New("not implemented")
}

func (m *mockShortener) CreateBatch(context.Context, []string, string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (m *mockShortener) Resolve(ctx context.Context, id string) (string, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, id)
	}
	return "", errors.New("not implemented")
}

func (m *mockShortener) FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error) {
	if m.findByUserIDFunc != nil {
		return m.findByUserIDFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockShortener) DeleteUserURLs(context.Context, string, []string) error {
	return errors.New("not implemented")
}

func TestGRPCHandlerUserWorkflow(t *testing.T) {
	var createdUserID string
	shortener := &mockShortener{
		createFunc: func(_ context.Context, url, userID string) (string, error) {
			if url != "https://example.com/long" || userID == "" {
				return "", errors.New("unexpected create arguments")
			}
			createdUserID = userID
			return "abc12345", nil
		},
		resolveFunc: func(_ context.Context, id string) (string, error) {
			if id != "abc12345" {
				return "", errors.New("unexpected id")
			}
			return "https://example.com/long", nil
		},
		findByUserIDFunc: func(_ context.Context, userID string) ([]model.UserURL, error) {
			if userID != createdUserID {
				return nil, errors.New("authorization user changed")
			}
			return []model.UserURL{{
				ShortID:     "abc12345",
				OriginalURL: "https://example.com/long",
			}}, nil
		},
	}
	client := newTestClient(t, shortener)

	var responseHeader metadata.MD
	shortenResponse, err := client.ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com/long"},
		grpc.Header(&responseHeader),
	)
	require.NoError(t, err)
	assert.Equal(t, "http://short.local/abc12345", shortenResponse.GetResult())
	require.NotEmpty(t, createdUserID)

	authorizationValues := responseHeader.Get("authorization")
	require.Len(t, authorizationValues, 1)
	require.NotEmpty(t, authorizationValues[0])
	authorizedContext := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", authorizationValues[0]),
	)

	listResponse, err := client.ListUserURLs(authorizedContext, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, listResponse.GetUrl(), 1)
	assert.Equal(t, "http://short.local/abc12345", listResponse.GetUrl()[0].GetShortUrl())
	assert.Equal(t, "https://example.com/long", listResponse.GetUrl()[0].GetOriginalUrl())

	expandResponse, err := client.ExpandURL(
		authorizedContext,
		&shortenerpb.URLExpandRequest{Id: "abc12345"},
	)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/long", expandResponse.GetResult())
}

func TestGRPCHandlerReturnsExistingShortURLOnConflict(t *testing.T) {
	client := newTestClient(t, &mockShortener{
		createFunc: func(context.Context, string, string) (string, error) {
			return "existing", service.ErrURLAlreadyExists
		},
	})

	response, err := client.ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com"},
	)

	require.NoError(t, err)
	assert.Equal(t, "http://short.local/existing", response.GetResult())
}

func TestGRPCHandlerAuthorizationAndValidationErrors(t *testing.T) {
	client := newTestClient(t, &mockShortener{
		createFunc: func(context.Context, string, string) (string, error) {
			return "", errors.New("store failed")
		},
		resolveFunc: func(context.Context, string) (string, error) {
			return "", service.ErrURLNotFound
		},
	})

	_, err := client.ShortenURL(context.Background(), &shortenerpb.URLShortenRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{Url: "https://example.com"},
	)
	assert.Equal(t, codes.Internal, status.Code(err))

	_, err = client.ExpandURL(context.Background(), &shortenerpb.URLExpandRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.ExpandURL(
		context.Background(),
		&shortenerpb.URLExpandRequest{Id: "missing"},
	)
	assert.Equal(t, codes.NotFound, status.Code(err))

	invalidAuthorizationContext := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", "broken"),
	)
	_, err = client.ListUserURLs(invalidAuthorizationContext, &emptypb.Empty{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGRPCShortenRejectsURLOverSharedLimit(t *testing.T) {
	serviceCalled := false
	client := newTestClient(t, &mockShortener{
		createFunc: func(context.Context, string, string) (string, error) {
			serviceCalled = true
			return "unexpected", nil
		},
	})

	_, err := client.ShortenURL(
		context.Background(),
		&shortenerpb.URLShortenRequest{
			Url: strings.Repeat("a", service.MaxOriginalURLLength+1),
		},
	)

	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.False(t, serviceCalled)
}

func TestGRPCExpandReturnsInternalForStorageFailure(t *testing.T) {
	client := newTestClient(t, &mockShortener{
		resolveFunc: func(context.Context, string) (string, error) {
			return "", errors.New("database unavailable")
		},
	})

	_, err := client.ExpandURL(
		context.Background(),
		&shortenerpb.URLExpandRequest{Id: "abc12345"},
	)

	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGRPCListCreatesAuthorizationForNewUser(t *testing.T) {
	client := newTestClient(t, &mockShortener{
		findByUserIDFunc: func(_ context.Context, userID string) ([]model.UserURL, error) {
			if userID == "" {
				return nil, errors.New("user id is empty")
			}
			return nil, nil
		},
	})

	var responseHeader metadata.MD
	response, err := client.ListUserURLs(
		context.Background(),
		&emptypb.Empty{},
		grpc.Header(&responseHeader),
	)

	require.NoError(t, err)
	assert.Empty(t, response.GetUrl())
	require.Len(t, responseHeader.Get("authorization"), 1)
}

func newTestClient(t *testing.T, shortener service.URLShortener) shortenerpb.ShortenerServiceClient {
	t.Helper()

	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	handler, err := grpcserver.NewHandler(shortener, "http://short.local", authenticator, nil)
	require.NoError(t, err)

	return newTestClientForHandler(t, handler)
}

func newTestClientForHandler(
	t *testing.T,
	handler shortenerpb.ShortenerServiceServer,
) shortenerpb.ShortenerServiceClient {
	t.Helper()

	listener := bufconn.Listen(bufconnSize)
	server := grpc.NewServer()
	shortenerpb.RegisterShortenerServiceServer(server, handler)
	go func() {
		_ = server.Serve(listener)
	}()

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, connection.Close())
		server.Stop()
		require.NoError(t, listener.Close())
	})

	return shortenerpb.NewShortenerServiceClient(connection)
}
