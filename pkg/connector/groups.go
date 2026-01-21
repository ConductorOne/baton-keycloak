package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type groupBuilder struct {
	resourceType  *v2.ResourceType
	client        *client.Client
	syncSubGroups bool
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource
	var groups []*client.Group
	var nextToken string
	var err error
	annos := annotations.Annotations{}

	switch {
	case parentResourceID == nil:
		// Top-level groups
		groups, nextToken, err = o.client.GetGroups(ctx, parseToken(pToken))
	case o.syncSubGroups:
		// Only fetch children if syncSubGroups is enabled
		groups, nextToken, err = o.client.GetGroupChildren(ctx, parentResourceID.Resource, parseToken(pToken))
	default:
		// syncSubGroups is disabled, don't fetch children
		return resources, "", annos, nil
	}

	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range groups {
		groupResource, err := parseIntoGroupResource(group, parentResourceID, o.syncSubGroups)
		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, groupResource)
	}

	return resources, nextToken, annos, nil
}

func (o *groupBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var entitlements []*v2.Entitlement

	membershipEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"member",
		entitlement.WithDisplayName(fmt.Sprintf("Membership in %s", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Membership in the %s group", resource.DisplayName)),
		entitlement.WithGrantableTo(userResourceType),
	)

	entitlements = append(entitlements, membershipEntitlement)
	return entitlements, "", nil, nil
}

func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grants []*v2.Grant
	annos := annotations.Annotations{}

	// Get all users in this group directly
	users, nextToken, err := o.client.GetGroupMembers(ctx, resource.Id.Resource, parseToken(pToken))
	if err != nil {
		return nil, "", nil, err
	}

	for _, user := range users {
		userResource, err := parseIntoUserResource(user, nil)
		if err != nil {
			return nil, "", nil, err
		}

		newGrant := grant.NewGrant(
			resource,
			"member",
			userResource,
		)

		grants = append(grants, newGrant)
	}

	if len(users) == 0 {
		nextToken = ""
	}

	return grants, nextToken, annos, nil
}

func (o *groupBuilder) Grant(ctx context.Context, resource *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	l.Info("Starting Grant operation",
		zap.String("resource_id", resource.Id.Resource),
		zap.String("resource_display_name", resource.DisplayName),
		zap.String("entitlement_id", entitlement.Id),
	)

	// Get the group ID from the entitlement ID
	groupID := entitlement.Resource.Id.Resource

	userID := resource.Id.Resource

	// Add user to group
	l.Info("Attempting to add user to group",
		zap.String("user_id", userID),
		zap.String("group_id", groupID),
	)

	if err := o.client.AddUserToGroup(ctx, userID, groupID); err != nil {
		l.Error("Failed to add user to group", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to add user to group: %w", err)
	}
	l.Info("Successfully added user to group")

	// Create and return the grant
	newGrant := grant.NewGrant(entitlement.Resource, "member", &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     userID,
		},
	})
	l.Info("Created grant", zap.String("grant_id", newGrant.Id))

	return []*v2.Grant{newGrant}, nil, nil
}

func (o *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	l.Info("Starting Revoke operation",
		zap.String("grant_id", grant.Id),
		zap.String("entitlement_id", grant.Entitlement.Id),
	)

	groupID := grant.Entitlement.Resource.Id.Resource
	userID := grant.Principal.Id.Resource

	// Remove user from group
	l.Info("Attempting to remove user from group",
		zap.String("user_id", userID),
		zap.String("group_id", groupID),
	)

	if err := o.client.RemoveUserFromGroup(ctx, userID, groupID); err != nil {
		l.Error("Failed to remove user from group", zap.Error(err))
		return nil, fmt.Errorf("failed to remove user from group: %w", err)
	}
	l.Info("Successfully removed user from group")

	return nil, nil
}

func parseIntoGroupResource(group *client.Group, parentResourceID *v2.ResourceId, syncSubGroups bool) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name": safeString(group.Name),
		"path": safeString(group.Path),
	}

	if group.Description != nil && *group.Description != "" {
		profile["description"] = *group.Description
	}

	if group.Attributes != nil {
		if desc, ok := (*group.Attributes)["description"]; ok && len(desc) > 0 {
			profile["description"] = desc[0]
		}
	}

	groupTraits := []resource.GroupTraitOption{
		resource.WithGroupProfile(profile),
	}

	var annotations []proto.Message
	// Only add ChildResourceType annotation if syncSubGroups is enabled and the group has children
	if syncSubGroups && group.SubGroupCount != nil && *group.SubGroupCount > 0 {
		annotations = append(annotations, &v2.ChildResourceType{
			ResourceTypeId: groupResourceType.Id,
		})
	}

	ret, err := resource.NewGroupResource(
		safeString(group.Name),
		groupResourceType,
		*group.ID,
		groupTraits,
		resource.WithParentResourceID(parentResourceID),
		resource.WithAnnotation(annotations...),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newGroupBuilder(client *client.Client, syncSubGroups bool) *groupBuilder {
	return &groupBuilder{
		resourceType:  groupResourceType,
		client:        client,
		syncSubGroups: syncSubGroups,
	}
}
