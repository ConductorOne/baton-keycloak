package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Nerzal/gocloak/v13"
	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestTokenErrorCode(t *testing.T) {
	// Mirror the real login chain: fmt.Errorf %w -> errors.Wrap -> *gocloak.APIError.
	wrapAPIErr := func(code int) error {
		apiErr := &gocloak.APIError{Code: code, Message: "boom"}
		return fmt.Errorf("failed to get token: %w", pkgerrors.Wrap(apiErr, "could not login to keycloak"))
	}

	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"401 unauthorized", wrapAPIErr(401), codes.Unauthenticated},
		{"400 bad request", wrapAPIErr(400), codes.Unauthenticated},
		{"403 forbidden", wrapAPIErr(403), codes.PermissionDenied},
		{"404 not found", wrapAPIErr(404), codes.Unauthenticated},
		{"429 too many requests", wrapAPIErr(429), codes.Unavailable},
		{"500 internal server error", wrapAPIErr(500), codes.Unavailable},
		{"non-APIError dial failure", errors.New("dial tcp: connection refused"), codes.Unavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tokenErrorCode(tt.err))
		})
	}
}
