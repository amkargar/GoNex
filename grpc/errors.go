package grpc

import (
	"context"
	"errors"
	apperrors "github.com/amkargar/gonex/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ToStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	var domain *apperrors.Error
	if errors.As(err, &domain) {
		code := codes.Internal
		switch domain.Code {
		case apperrors.NotFound:
			code = codes.NotFound
		case apperrors.Conflict:
			code = codes.AlreadyExists
		case apperrors.Invalid:
			code = codes.InvalidArgument
		case apperrors.Unavailable:
			code = codes.Unavailable
		case apperrors.Unauthorized:
			code = codes.Unauthenticated
		case apperrors.Forbidden:
			code = codes.PermissionDenied
		}
		if code != codes.Internal {
			return status.Error(code, domain.Message)
		}
	}
	return status.Error(codes.Internal, "internal server error")
}
func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.NotFound:
		return apperrors.New(apperrors.NotFound, "remote record not found")
	case codes.AlreadyExists:
		return apperrors.New(apperrors.Conflict, "remote record already exists")
	case codes.InvalidArgument:
		return apperrors.New(apperrors.Invalid, "invalid remote request")
	case codes.Unauthenticated:
		return apperrors.New(apperrors.Unauthorized, "remote authentication required")
	case codes.PermissionDenied:
		return apperrors.New(apperrors.Forbidden, "remote access denied")
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return apperrors.New(apperrors.Unavailable, "remote service unavailable")
	}
}
