package pricing

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// ErrUnsupportedResourceType is returned when the provider is not "azure"
// or the resource type has no defined mapping to an Azure service name.
// Maps to gRPC codes.Unimplemented via MapToGRPCStatus.
var ErrUnsupportedResourceType = errors.New("unsupported resource type")

// ErrMissingRequiredFields is returned when region and/or SKU cannot be
// resolved from primary descriptor fields or tag fallback.
// Maps to gRPC codes.InvalidArgument via MapToGRPCStatus.
var ErrMissingRequiredFields = errors.New("missing required fields")

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
	case errors.Is(err, ErrUnsupportedResourceType):
		code = codes.Unimplemented
	case errors.Is(err, ErrMissingRequiredFields):
		code = codes.InvalidArgument
	}

	return status.New(code, err.Error())
}
