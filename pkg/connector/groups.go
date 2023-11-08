package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	sdkEntitlement "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	sdkGrant "github.com/conductorone/baton-sdk/pkg/types/grant"
	sdkResource "github.com/conductorone/baton-sdk/pkg/types/resource"
)

type groupBuilder struct {
	client *client.Client
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) Fetch(ctx context.Context, resourceId *v2.ResourceId) (*v2.Resource, annotations.Annotations, error) {
	//var ret *v2.Resource
	if resourceId.ResourceType != groupResourceType.Id {
		return nil, nil, fmt.Errorf("invalid resource type")
	}
	group, err := o.client.GetGroup(ctx, resourceId.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get group")
	}
	ret, err := sdkResource.NewGroupResource(group.Name, o.ResourceType(ctx), group.Id, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create new group resource")
	}

	return ret, nil, nil
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var ret []*v2.Resource
	groups, err := o.client.ListGroups(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range groups {
		ur, err := sdkResource.NewGroupResource(group.Name, o.ResourceType(ctx), group.Id, nil)
		if err != nil {
			return nil, "", nil, err
		}

		ret = append(ret, ur)
	}
	return ret, "", nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var ret []*v2.Entitlement
	groupEntitlement := sdkEntitlement.NewAssignmentEntitlement(resource, "member", sdkEntitlement.WithGrantableTo(userResourceType))
	ret = append(ret, groupEntitlement)
	return ret, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var ret []*v2.Grant

	groupMembers, err := o.client.ListGroupMembers(ctx, resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}

	for _, member := range groupMembers {
		grant := sdkGrant.NewGrant(resource, "member", &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     member.Id,
		})
		ret = append(ret, grant)
	}

	return ret, "", nil, nil
}
func newGroupBuilder(client *client.Client) *groupBuilder {
	return &groupBuilder{client}
}
