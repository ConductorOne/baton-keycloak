package client

import (
	"errors"
	"net/http"

	"github.com/Nerzal/gocloak/v13"
)

// IsAlreadyExistsError reports whether err is a Keycloak 409 Conflict. The Admin
// REST API returns 409 from POST .../users when the username or email already
// exists, which CreateAccount must treat as success (AlreadyExistsResult).
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
