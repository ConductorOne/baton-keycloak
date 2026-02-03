package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type groupBuilder struct {
	resourceType    *v2.ResourceType
	client          *client.Client
	syncSubGroups   bool
	keycloakVersion int
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	var resources []*v2.Resource
	annos := annotations.Annotations{}

	// If we have a parent, we're fetching sub-groups
	if parentResourceID != nil {
		if o.keycloakVersion >= 23 {
			// Keycloak 23+ supports the /children endpoint
			groups, nextToken, err := o.client.GetGroupChildren(ctx, parentResourceID.Resource, parseToken(&attrs.PageToken))
			if err != nil {
				return nil, nil, err
			}

			for _, group := range groups {
				groupResource, err := o.parseIntoGroupResource(group, parentResourceID)
				if err != nil {
					return nil, nil, err
				}
				resources = append(resources, groupResource)
			}

			return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
		} else {
			// Keycloak < 23: sub-groups are embedded in parent group response
			// This case should not happen in List() since we flatten at top level
			// But return empty for safety
			return resources, &rs.SyncOpResults{Annotations: annos}, nil
		}
	}

	// Fetch top-level groups
	groups, nextToken, err := o.client.GetGroups(ctx, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	for _, group := range groups {
		if o.syncSubGroups {
			if o.keycloakVersion >= 23 {
				// Keycloak 23+: Create resource with ChildResourceType annotation if it has children
				// The SDK will call List() again with parentResourceID to fetch children
				groupResource, err := o.parseIntoGroupResource(group, nil)
				if err != nil {
					return nil, nil, err
				}
				resources = append(resources, groupResource)
			} else {
				// Keycloak < 23: sub-groups come embedded in the response
				// Recursively flatten all groups (top-level and all sub-groups) into resources
				groupResources, err := o.flattenGroupHierarchy(group, nil)
				if err != nil {
					return nil, nil, err
				}
				resources = append(resources, groupResources...)
			}
		} else {
			// Only sync top-level groups
			groupResource, err := o.parseIntoGroupResource(group, nil)
			if err != nil {
				return nil, nil, err
			}
			resources = append(resources, groupResource)
		}
	}

	if len(groups) == 0 {
		nextToken = ""
	}

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

func (o *groupBuilder) Entitlements(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var entitlements []*v2.Entitlement

	// Membership is grantable to both users and groups (for hierarchical group expansion)
	grantableTo := []*v2.ResourceType{userResourceType}
	if o.syncSubGroups {
		grantableTo = append(grantableTo, groupResourceType)
	}

	membershipEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"member",
		entitlement.WithDisplayName(fmt.Sprintf("Membership in %s", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Membership in the %s group", resource.DisplayName)),
		entitlement.WithGrantableTo(grantableTo...),
	)

	entitlements = append(entitlements, membershipEntitlement)
	return entitlements, nil, nil
}

func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	var grants []*v2.Grant
	annos := annotations.Annotations{}
	// On the first page only, emit a grant for hierarchical group membership.
	// This grant represents "this subgroup is a member of its parent group".
	// The GrantExpandable annotation enables expansion so that users who are members
	// of this subgroup will also get membership in the parent group.
	if attrs.PageToken.Token == "" && o.syncSubGroups {
		parentResourceID := resource.GetParentResourceId()
		if parentResourceID != nil {
			// Build this group's membership entitlement ID for the GrantExpandable annotation
			// Format: group:{group_id}:member
			thisGroupMemberEntitlementID := entitlement.NewEntitlementID(resource, "member")

			// Create parent resource reference
			parentResource := &v2.Resource{
				Id: parentResourceID,
			}

			// Create grant: this subgroup is a member of its parent group
			// With GrantExpandable: anyone who has thisGroup:member should also get parentGroup:member
			parentMembershipGrant := grant.NewGrant(
				parentResource,
				"member",
				resource, // This group is the principal (member of parent)
				grant.WithAnnotation(&v2.GrantExpandable{
					EntitlementIds: []string{thisGroupMemberEntitlementID},
				}),
			)
			grants = append(grants, parentMembershipGrant)
		}
	}

	// Get all users in this group directly
	users, nextToken, err := o.client.GetGroupMembers(ctx, resource.Id.Resource, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	for _, user := range users {
		userResource, err := parseIntoUserResource(user, nil)
		if err != nil {
			return nil, nil, err
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

	return grants, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

func (o *groupBuilder) Grant(ctx context.Context, resource *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	l.Info("Starting Grant operation",
		zap.String("resource_id", resource.Id.Resource),
		zap.String("resource_display_name", resource.DisplayName),
		zap.String("entitlement_id", entitlement.Id),
	)

	if resource.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("cannot grant entitlement on non-user resource")
	}

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

	if grant.Entitlement.Resource.Id.ResourceType != groupResourceType.Id {
		return nil, fmt.Errorf("cannot revoke entitlement on non-group resource")
	}

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

// flattenGroupHierarchy recursively converts a group and all its sub-groups into resources.
func (o *groupBuilder) flattenGroupHierarchy(group *client.Group, parentResourceID *v2.ResourceId) ([]*v2.Resource, error) {
	var resources []*v2.Resource

	// Create resource for this group
	// Note: syncSubGroups is always true when flattenGroupHierarchy is called
	groupResource, err := o.parseIntoGroupResource(group, parentResourceID)
	if err != nil {
		return nil, err
	}
	resources = append(resources, groupResource)

	// Recursively process sub-groups
	if group.SubGroups != nil && len(*group.SubGroups) > 0 {
		thisGroupID := &v2.ResourceId{
			ResourceType: groupResourceType.Id,
			Resource:     *group.ID,
		}
		for _, subGroup := range *group.SubGroups {
			subGroupCopy := subGroup
			subResources, err := o.flattenGroupHierarchy(&subGroupCopy, thisGroupID)
			if err != nil {
				return nil, err
			}
			resources = append(resources, subResources...)
		}
	}

	return resources, nil
}

func (o *groupBuilder) parseIntoGroupResource(group *client.Group, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name": safeString(group.Name),
		"path": safeString(group.Path),
	}

	if group.Attributes != nil {
		if desc, ok := (*group.Attributes)["description"]; ok && len(desc) > 0 {
			profile["description"] = desc[0]
		}
	}

	groupTraits := []rs.GroupTraitOption{
		rs.WithGroupProfile(profile),
	}

	var annotations []proto.Message
	// Add ChildResourceType annotation if syncSubGroups is enabled and the group has children
	if o.syncSubGroups && group.SubGroupsCount != nil && *group.SubGroupsCount > 0 {
		annotations = append(annotations, &v2.ChildResourceType{
			ResourceTypeId: groupResourceType.Id,
		})
	}

	ret, err := rs.NewGroupResource(
		safeString(group.Name),
		groupResourceType,
		*group.ID,
		groupTraits,
		rs.WithParentResourceID(parentResourceID),
		rs.WithAnnotation(annotations...),
	)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newGroupBuilder(client *client.Client, syncSubGroups bool, keycloakVersion int) *groupBuilder {
	return &groupBuilder{
		resourceType:    groupResourceType,
		client:          client,
		syncSubGroups:   syncSubGroups,
		keycloakVersion: keycloakVersion,
	}
}
