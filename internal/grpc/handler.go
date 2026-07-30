// Package grpcserver реализует gRPC-транспорт сервиса сокращения URL.
package grpcserver

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/audit"
	"github.com/timaogurtzova/shortener/internal/auth"
	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/service"
	shortenerpb "github.com/timaogurtzova/shortener/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const authorizationMetadataKey = "authorization"

// AuditNotifier принимает события аудита от gRPC-хендлера.
type AuditNotifier interface {
	Notify(context.Context, audit.Event) error
}

// Handler преобразует gRPC-запросы в вызовы общего сервисного слоя.
type Handler struct {
	shortenerpb.UnimplementedShortenerServiceServer

	service       service.URLShortener
	baseURL       string
	authenticator *auth.Authenticator
	auditor       AuditNotifier
}

// NewHandler создаёт gRPC-фасад над сервисом сокращения URL.
func NewHandler(
	shortener service.URLShortener,
	baseURL string,
	authenticator *auth.Authenticator,
	auditor AuditNotifier,
) (*Handler, error) {
	if shortener == nil {
		return nil, errors.New("shortener service is nil")
	}
	if authenticator == nil {
		return nil, errors.New("authenticator is nil")
	}

	return &Handler{
		service:       shortener,
		baseURL:       strings.TrimRight(baseURL, "/"),
		authenticator: authenticator,
		auditor:       auditor,
	}, nil
}

// ShortenURL сокращает URL и возвращает полный короткий адрес.
func (h *Handler) ShortenURL(
	ctx context.Context,
	request *shortenerpb.URLShortenRequest,
) (*shortenerpb.URLShortenResponse, error) {
	originalURL, err := service.NormalizeOriginalURL(request.GetUrl())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	userID, newAuthorization, err := h.authenticator.EnsureAuthorization(authorizationFromContext(ctx))
	if err != nil {
		return nil, internalRPCError("ShortenURL", "authorize user", err)
	}
	if headerErr := setAuthorizationHeader(ctx, newAuthorization); headerErr != nil {
		return nil, internalRPCError("ShortenURL", "set authorization metadata", headerErr)
	}

	shortID, err := h.service.Create(ctx, originalURL, userID)
	if err != nil && !errors.Is(err, service.ErrURLAlreadyExists) {
		if errors.Is(err, service.ErrURLInvalid) || errors.Is(err, service.ErrURLTooLong) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, internalRPCError("ShortenURL", "shorten url", err)
	}
	if shortID == "" {
		return nil, internalRPCError("ShortenURL", "short url is empty", errors.New("service returned an empty short id"))
	}

	if err == nil {
		h.publishAudit(ctx, audit.ActionShorten, userID, originalURL)
	}

	return &shortenerpb.URLShortenResponse{
		Result: h.baseURL + "/" + shortID,
	}, nil
}

// ExpandURL возвращает исходный URL по короткому идентификатору.
func (h *Handler) ExpandURL(
	ctx context.Context,
	request *shortenerpb.URLExpandRequest,
) (*shortenerpb.URLExpandResponse, error) {
	shortID := strings.TrimSpace(request.GetId())
	if shortID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is empty")
	}

	originalURL, err := h.service.Resolve(ctx, shortID)
	if err != nil {
		if errors.Is(err, service.ErrURLNotFound) || errors.Is(err, service.ErrURLDeleted) {
			return nil, status.Error(codes.NotFound, "url not found")
		}
		return nil, internalRPCError("ExpandURL", "expand url", err)
	}

	userID, _ := h.authenticator.AuthorizationUserID(authorizationFromContext(ctx))
	h.publishAudit(ctx, audit.ActionFollow, userID, originalURL)

	return &shortenerpb.URLExpandResponse{Result: originalURL}, nil
}

// ListUserURLs возвращает все URL авторизованного пользователя.
func (h *Handler) ListUserURLs(
	ctx context.Context,
	_ *emptypb.Empty,
) (*shortenerpb.UserURLsResponse, error) {
	userID, newAuthorization, err := h.authenticator.AuthorizationForHistory(authorizationFromContext(ctx))
	if err != nil {
		if errors.Is(err, auth.ErrAuthorizationInvalid) {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization")
		}
		return nil, internalRPCError("ListUserURLs", "authorize user", err)
	}
	if headerErr := setAuthorizationHeader(ctx, newAuthorization); headerErr != nil {
		return nil, internalRPCError("ListUserURLs", "set authorization metadata", headerErr)
	}

	userURLs, err := h.service.FindByUserID(ctx, userID)
	if err != nil {
		return nil, internalRPCError("ListUserURLs", "list user urls", err)
	}

	return &shortenerpb.UserURLsResponse{
		Url: h.userURLData(userURLs),
	}, nil
}

func (h *Handler) userURLData(userURLs []model.UserURL) []*shortenerpb.URLData {
	result := make([]*shortenerpb.URLData, len(userURLs))
	for i, item := range userURLs {
		result[i] = &shortenerpb.URLData{
			ShortUrl:    h.baseURL + "/" + item.ShortID,
			OriginalUrl: item.OriginalURL,
		}
	}

	return result
}

func (h *Handler) publishAudit(ctx context.Context, action audit.Action, userID, originalURL string) {
	if h.auditor == nil {
		return
	}

	if err := h.auditor.Notify(ctx, audit.NewEvent(action, userID, originalURL)); err != nil {
		log.Warn().Err(err).Msg("failed to enqueue gRPC audit event")
	}
}

func authorizationFromContext(ctx context.Context) string {
	incomingMetadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := incomingMetadata.Get(authorizationMetadataKey)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func setAuthorizationHeader(ctx context.Context, authorization string) error {
	if authorization == "" {
		return nil
	}

	return grpc.SetHeader(ctx, metadata.Pairs(authorizationMetadataKey, authorization))
}

func internalRPCError(method, message string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, context.Canceled.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	}

	log.Error().
		Err(err).
		Str("method", method).
		Msg(message)
	return status.Error(codes.Internal, message)
}
