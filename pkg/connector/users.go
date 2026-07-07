package connector

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/crypto"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// requiredActionUpdatePassword forces a NO_PASSWORD user to set a credential at first login.
const requiredActionUpdatePassword = "UPDATE_PASSWORD"

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

// CreateAccountCapabilityDetails declares the credential options supported when provisioning an account (RANDOM_PASSWORD preferred).
func (o *userBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
	}, nil, nil
}

// CreateAccount provisions a new user in the realm, setting the credential inline
// on the create call. A duplicate username/email (409) is returned as AlreadyExistsResult.
func (o *userBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.LocalCredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	if credentialOptions == nil {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "baton-keycloak: create-account: missing credential options")
	}

	profileMap := accountInfo.GetProfile().AsMap()

	username := extractUsername(accountInfo, profileMap)
	if username == "" {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "baton-keycloak: create-account: username is required")
	}

	email, _ := profileMap["email"].(string)
	if email == "" {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "baton-keycloak: create-account %s: email is required", username)
	}

	firstName, _ := profileMap["firstName"].(string)
	lastName, _ := profileMap["lastName"].(string)

	newUser := gocloak.User{
		Username:  gocloak.StringP(username),
		Enabled:   gocloak.BoolP(true),
		Email:     gocloak.StringP(email),
		FirstName: stringPOrNil(firstName),
		LastName:  stringPOrNil(lastName),
	}

	// NO_PASSWORD attaches a one-time UPDATE_PASSWORD action; otherwise set the password inline.
	var ptds []*v2.PlaintextData
	if credentialOptions.GetNoPassword() != nil {
		newUser.RequiredActions = &[]string{requiredActionUpdatePassword}
	} else {
		password, err := crypto.GeneratePassword(ctx, credentialOptions)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("baton-keycloak: create-account %s: failed to generate password: %w", username, err)
		}
		newUser.Credentials = &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     gocloak.StringP(password),
				Temporary: gocloak.BoolP(false),
			},
		}
		ptds = []*v2.PlaintextData{
			{
				Name:  "password",
				Bytes: []byte(password),
			},
		}
	}

	userID, err := o.client.CreateUser(ctx, newUser)
	alreadyExists := client.IsAlreadyExistsError(err)
	if err != nil && !alreadyExists {
		return nil, nil, nil, fmt.Errorf("baton-keycloak: create-account %s: %w", username, err)
	}

	// Read back to build the resource: by ID for a fresh create, by username/email after a 409 (no ID returned).
	var user *gocloak.User
	if userID != "" {
		user, err = o.client.GetUserByID(ctx, userID)
	} else {
		user, err = o.resolveExistingUser(ctx, username, email)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-keycloak: create-account %s: %w", username, err)
	}

	ur, err := parseIntoUserResource(user, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-keycloak: create-account %s: %w", username, err)
	}

	if alreadyExists {
		return &v2.CreateAccountResponse_AlreadyExistsResult{Resource: ur}, nil, nil, nil
	}

	return &v2.CreateAccountResponse_SuccessResult{Resource: ur}, ptds, nil, nil
}

// Delete hard-deletes a Keycloak user; a missing user (404) is treated as success.
func (o *userBuilder) Delete(ctx context.Context, resourceID *v2.ResourceId) (annotations.Annotations, error) {
	if resourceID.GetResourceType() != userResourceType.Id {
		return nil, status.Errorf(codes.InvalidArgument, "baton-keycloak: delete: invalid resource type %q, expected %q", resourceID.GetResourceType(), userResourceType.Id)
	}

	userID := resourceID.GetResource()
	if err := o.client.DeleteUser(ctx, userID); err != nil {
		if client.IsNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("baton-keycloak: delete user %s: %w", userID, err)
	}

	return nil, nil
}

// resolveExistingUser looks up a user after a create conflict, matching by username then email.
func (o *userBuilder) resolveExistingUser(ctx context.Context, username, email string) (*gocloak.User, error) {
	user, err := o.client.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil && email != "" {
		user, err = o.client.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
	}
	if user == nil {
		return nil, fmt.Errorf("user %q not found after create conflict", username)
	}
	return user, nil
}

// extractUsername resolves the username: the schema-declared "username" profile
// field wins, falling back to the account login only when it is empty.
func extractUsername(accountInfo *v2.AccountInfo, profileMap map[string]any) string {
	if username, ok := profileMap["username"].(string); ok && username != "" {
		return username
	}
	if login := accountInfo.GetLogin(); login != "" {
		return login
	}
	return ""
}

// stringPOrNil returns a pointer to s, or nil when s is empty.
func stringPOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// newUserBuilder creates a new instance of userBuilder.
// This is the constructor function for the userBuilder struct.
func newUserBuilder(client *client.Client) *userBuilder {
	return &userBuilder{
		resourceType: userResourceType,
		client:       client,
	}
}

// parseIntoUserResource converts a Keycloak user object into a Baton SDK user resource.
func parseIntoUserResource(user *gocloak.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	username := safeString(user.Username)
	email := safeString(user.Email)

	// Reflect the Keycloak enabled flag as account status (disabled → STATUS_DISABLED).
	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	if user.Enabled != nil && !*user.Enabled {
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	profile := map[string]any{
		"username":  username,
		"email":     email,
		"firstName": safeString(user.FirstName),
		"lastName":  safeString(user.LastName),
	}

	userTraits := []rs.UserTraitOption{
		rs.WithUserProfile(profile),
		rs.WithUserLogin(username),
		rs.WithEmail(email, true),
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
