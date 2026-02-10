package pricing

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// MapToGRPCStatus maps an azureclient error to a gRPC status.
// The error message is preserved in the gRPC status message.
// Mapping is evaluated via errors.Is in priority order.
func MapToGRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	code := codes.Internal
	switch {
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	case errors.Is(err, azureclient.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, azureclient.ErrRateLimited):
		code = codes.ResourceExhausted
	case errors.Is(err, azureclient.ErrServiceUnavailable):
		code = codes.Unavailable
	case errors.Is(err, azureclient.ErrRequestFailed):
		code = codes.Internal
	case errors.Is(err, azureclient.ErrInvalidResponse):
		code = codes.Internal
	case errors.Is(err, azureclient.ErrInvalidConfig):
		code = codes.Internal
	case errors.Is(err, azureclient.ErrPaginationLimitExceeded):
		code = codes.Internal
	}

	return status.New(code, err.Error())
}
