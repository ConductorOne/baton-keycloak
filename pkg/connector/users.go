package connector

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"

	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// userBuilder implements the resource builder interface for Keycloak user resources.
// It handles the creation and synchronization of user resources between Keycloak and Baton.
type userBuilder struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

// ResourceType returns the v2.ResourceType for users.
// This identifies the type of resources this builder manages.
func (o *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

// List retrieves all user resources from Keycloak and converts them to the Baton format.
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - parentResourceID: The parent resource ID (unused in this implementation)
//   - pToken: Pagination token for handling large result sets
//
// Returns:
//   - []*v2.Resource: List of user resources
//   - string: Next page token for pagination
//   - annotations.Annotations: Additional metadata
//   - error: Any error that occurred during the operation
func (o *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	annos := annotations.Annotations{}

	users, nextToken, err := o.client.GetUsers(ctx, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	resource := make([]*v2.Resource, 0, len(users))
	for _, user := range users {
		userResource, err := parseIntoUserResource(user, nil)
		if err != nil {
			return nil, nil, err
		}
		resource = append(resource, userResource)
	}

	if len(users) == 0 {
		nextToken = ""
	}

	return resource, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

// Entitlements returns entitlements for the user resource.
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - resource: The user resource
//   - pToken: Pagination token for handling large result sets
//
// Returns:
//   - []*v2.Entitlement: List of entitlements for the user
//   - string: Next page token for pagination
//   - annotations.Annotations: Additional metadata
//   - error: Any error that occurred during the operation
func (o *userBuilder) Entitlements(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants returns grants for the user resource.
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - resource: The user resource
//   - pToken: Pagination token for handling large result sets
//
// Returns:
//   - []*v2.Grant: List of grants for the user
//   - string: Next page token for pagination
//   - annotations.Annotations: Additional metadata
//   - error: Any error that occurred during the operation
func (o *userBuilder) Grants(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// newUserBuilder creates a new instance of userBuilder.
// This is the constructor function for the userBuilder struct.
func newUserBuilder(client *client.Client) *userBuilder {
	return &userBuilder{
		resourceType: userResourceType,
		client:       client,
	}
}

var _ connectorbuilder.AccountManagerV2 = (*userBuilder)(nil)

func (o *userBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.LocalCredentialOptions,
) (connectorbuilder.CreateAccountResponse, []*v2.PlaintextData, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	login := accountInfo.GetLogin()
	if login == "" {
		return nil, nil, nil, fmt.Errorf("baton-keycloak: login is required for account creation")
	}

	var email string
	for _, e := range accountInfo.GetEmails() {
		if e.GetIsPrimary() {
			email = e.GetAddress()
			break
		}
	}
	if email == "" && len(accountInfo.GetEmails()) > 0 {
		email = accountInfo.GetEmails()[0].GetAddress()
	}

	var firstName, lastName string
	if profile := accountInfo.GetProfile(); profile != nil {
		if v, ok := profile.GetFields()["first_name"]; ok {
			firstName = v.GetStringValue()
		}
		if v, ok := profile.GetFields()["last_name"]; ok {
			lastName = v.GetStringValue()
		}
	}

	user := gocloak.User{
		Username:      &login,
		Enabled:       pointer(true),
		EmailVerified: pointer(true),
	}
	if email != "" {
		user.Email = &email
	}
	if firstName != "" {
		user.FirstName = &firstName
	}
	if lastName != "" {
		user.LastName = &lastName
	}

	var plaintextData []*v2.PlaintextData
	if credentialOptions != nil {
		if pw := credentialOptions.GetPlaintextPassword(); pw != nil {
			password := pw.GetPlaintextPassword()
			user.Credentials = &[]gocloak.CredentialRepresentation{{
				Type:      pointer("password"),
				Value:     &password,
				Temporary: pointer(credentialOptions.GetForceChangeAtNextLogin()),
			}}
			plaintextData = append(plaintextData, v2.PlaintextData_builder{
				Name:  "password",
				Bytes: []byte(password),
			}.Build())
		} else if rp := credentialOptions.GetRandomPassword(); rp != nil {
			length := rp.GetLength()
			if length <= 0 {
				length = 24
			}
			password, err := generateRandomPassword(int(length))
			if err != nil {
				return nil, nil, nil, fmt.Errorf("baton-keycloak: failed to generate random password: %w", err)
			}
			user.Credentials = &[]gocloak.CredentialRepresentation{{
				Type:      pointer("password"),
				Value:     &password,
				Temporary: pointer(credentialOptions.GetForceChangeAtNextLogin()),
			}}
			plaintextData = append(plaintextData, v2.PlaintextData_builder{
				Name:  "password",
				Bytes: []byte(password),
			}.Build())
		}
	}

	l.Info("Creating user in Keycloak", zap.String("username", login))

	userID, err := o.client.CreateUser(ctx, user)
	if err != nil {
		var apiErr *gocloak.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusConflict {
			l.Info("User already exists in Keycloak", zap.String("username", login))
			existingUsers, lookupErr := o.client.GetUsersByUsername(ctx, login)
			if lookupErr != nil || len(existingUsers) == 0 {
				return nil, nil, nil, fmt.Errorf("baton-keycloak: user already exists but could not be retrieved: %w", err)
			}
			resource, parseErr := parseIntoUserResource(existingUsers[0], nil)
			if parseErr != nil {
				return nil, nil, nil, fmt.Errorf("baton-keycloak: failed to parse existing user: %w", parseErr)
			}
			return v2.CreateAccountResponse_AlreadyExistsResult_builder{
				Resource:              resource,
				IsCreateAccountResult: true,
			}.Build(), nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("baton-keycloak: failed to create user: %w", err)
	}

	l.Info("Successfully created user in Keycloak", zap.String("user_id", userID), zap.String("username", login))

	createdUser := &gocloak.User{
		ID:            &userID,
		Username:      &login,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Enabled:       pointer(true),
		EmailVerified: pointer(true),
	}

	resource, err := parseIntoUserResource(createdUser, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-keycloak: failed to parse created user resource: %w", err)
	}

	return v2.CreateAccountResponse_SuccessResult_builder{
		Resource:              resource,
		IsCreateAccountResult: true,
	}.Build(), plaintextData, nil, nil
}

func (o *userBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return v2.CredentialDetailsAccountProvisioning_builder{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}.Build(), nil, nil
}

func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}
	return string(result), nil
}

// parseIntoUserResource converts a Linode user object into a Baton SDK user resource.
// Parameters:
//   - ctx: Context (currently unused)
//   - user: Pointer to the Linode user object to convert
//   - parentResourceID: Optional parent resource ID for hierarchy
//
// Returns:
//   - *v2.Resource: The converted Baton resource
//   - error: Any conversion error that occurred
func parseIntoUserResource(user *gocloak.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_ENABLED
	username := safeString(user.Username)

	profile := map[string]interface{}{
		"username":  username,
		"email":     safeString(user.Email),
		"firstName": safeString(user.FirstName),
		"lastName":  safeString(user.LastName),
	}

	userTraits := []rs.UserTraitOption{
		rs.WithUserProfile(profile),
		rs.WithUserLogin(username),
		rs.WithStatus(userStatus),
	}

	ret, err := rs.NewUserResource(
		username,
		userResourceType,
		safeString(user.ID),
		userTraits,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pointer[T any](v T) *T {
	return &v
}
