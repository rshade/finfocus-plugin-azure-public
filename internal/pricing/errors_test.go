package pricing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestMapToGRPCStatus(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
	}{
		{
			name:         "nil error returns OK",
			err:          nil,
			expectedCode: codes.OK,
		},
		{
			name:         "context.Canceled maps to Canceled",
			err:          context.Canceled,
			expectedCode: codes.Canceled,
		},
		{
			name:         "context.DeadlineExceeded maps to DeadlineExceeded",
			err:          context.DeadlineExceeded,
			expectedCode: codes.DeadlineExceeded,
		},
		{
			name:         "ErrNotFound maps to NotFound",
			err:          azureclient.ErrNotFound,
			expectedCode: codes.NotFound,
		},
		{
			name:         "ErrRateLimited maps to ResourceExhausted",
			err:          azureclient.ErrRateLimited,
			expectedCode: codes.ResourceExhausted,
		},
		{
			name:         "ErrServiceUnavailable maps to Unavailable",
			err:          azureclient.ErrServiceUnavailable,
			expectedCode: codes.Unavailable,
		},
		{
			name:         "ErrRequestFailed maps to Internal",
			err:          azureclient.ErrRequestFailed,
			expectedCode: codes.Internal,
		},
		{
			name:         "ErrInvalidResponse maps to Internal",
			err:          azureclient.ErrInvalidResponse,
			expectedCode: codes.Internal,
		},
		{
			name:         "ErrInvalidConfig maps to Internal",
			err:          azureclient.ErrInvalidConfig,
			expectedCode: codes.Internal,
		},
		{
			name:         "ErrPaginationLimitExceeded maps to Internal",
			err:          azureclient.ErrPaginationLimitExceeded,
			expectedCode: codes.Internal,
		},
		{
			name:         "unknown error maps to Internal",
			err:          errors.New("unknown"),
			expectedCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := MapToGRPCStatus(tt.err)
			if s.Code() != tt.expectedCode {
				t.Errorf("expected code %v, got %v", tt.expectedCode, s.Code())
			}
		})
	}
}

func TestMapToGRPCStatus_WrappedErrors(t *testing.T) {
	wrappedNotFound := fmt.Errorf("wrapped: %w", azureclient.ErrNotFound)
	s := MapToGRPCStatus(wrappedNotFound)
	if s.Code() != codes.NotFound {
		t.Errorf("expected NotFound for wrapped ErrNotFound, got %v", s.Code())
	}
	if s.Message() != wrappedNotFound.Error() {
		t.Errorf("expected message %q, got %q", wrappedNotFound.Error(), s.Message())
	}
}

func TestMapToGRPCStatus_PreservesErrorMessage(t *testing.T) {
	err := fmt.Errorf("%w: status 429: too many requests", azureclient.ErrRateLimited)
	s := MapToGRPCStatus(err)
	if s.Code() != codes.ResourceExhausted {
		t.Errorf("expected ResourceExhausted, got %v", s.Code())
	}
	if s.Message() != err.Error() {
		t.Errorf("expected message %q, got %q", err.Error(), s.Message())
	}
}
