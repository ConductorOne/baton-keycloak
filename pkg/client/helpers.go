package client

import (
	"errors"
	"net/http"

	"github.com/Nerzal/gocloak/v13"
	"google.golang.org/grpc/codes"
)

// IsAlreadyExistsError reports whether err is a Keycloak 409 Conflict (duplicate username/email on create).
func IsAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *gocloak.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusConflict
	}
	return false
}

// IsNotFoundError reports whether err is a Keycloak 404 Not Found.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *gocloak.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusNotFound
	}
	return false
}

// MapAPIError maps a Keycloak Admin API error to the closest gRPC status code.
// Returns codes.OK for a nil error and codes.Unknown for a non-*gocloak.APIError.
// Pair with uhttp.WrapErrors so the original error stays in the chain.
func MapAPIError(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	var apiErr *gocloak.APIError
	if !errors.As(err, &apiErr) {
		return codes.Unknown
	}
	switch {
	case apiErr.Code == http.StatusBadRequest:
		return codes.InvalidArgument
	case apiErr.Code == http.StatusUnauthorized:
		return codes.Unauthenticated
	case apiErr.Code == http.StatusForbidden:
		return codes.PermissionDenied
	case apiErr.Code == http.StatusNotFound:
		return codes.NotFound
	case apiErr.Code == http.StatusConflict:
		return codes.AlreadyExists
	case apiErr.Code == http.StatusTooManyRequests, apiErr.Code >= http.StatusInternalServerError:
		return codes.Unavailable
	default:
		return codes.Unknown
	}
}
