package connector

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
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
