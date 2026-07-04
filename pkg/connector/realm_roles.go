package connector

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type realmRoleBuilder struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

func (o *realmRoleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return realmRoleResourceType
}

func (o *realmRoleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	roles, nextToken, err := o.client.GetRealmRoles(ctx, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	var resources []*v2.Resource
	for _, role := range roles {
		resource, err := parseIntoRealmRoleResource(role)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, resource)
	}

	if len(roles) == 0 {
		nextToken = ""
	}

	return resources, &rs.SyncOpResults{NextPageToken: nextToken}, nil
}

func (o *realmRoleBuilder) Entitlements(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	assignmentEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"assigned",
		entitlement.WithDisplayName(fmt.Sprintf("Assigned %s", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Assigned the %s realm role", resource.DisplayName)),
		entitlement.WithGrantableTo(userResourceType),
	)

	return []*v2.Entitlement{assignmentEntitlement}, nil, nil
}

func (o *realmRoleBuilder) Grants(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	roleName := resource.DisplayName

	users, nextToken, err := o.client.GetUsersByRealmRoleName(ctx, roleName, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	var grants []*v2.Grant
	for _, user := range users {
		userResource, err := parseIntoUserResource(user, nil)
		if err != nil {
			return nil, nil, err
		}

		newGrant := grant.NewGrant(
			resource,
			"assigned",
			userResource,
		)
		grants = append(grants, newGrant)
	}

	if len(users) == 0 {
		nextToken = ""
	}

	return grants, &rs.SyncOpResults{NextPageToken: nextToken}, nil
}

func (o *realmRoleBuilder) Grant(ctx context.Context, resource *v2.Resource, ent *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if resource.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("baton-keycloak: cannot grant realm role to non-user resource")
	}

	roleID := ent.Resource.Id.Resource
	roleName := ent.Resource.DisplayName
	userID := resource.Id.Resource

	l.Info("Granting realm role to user",
		zap.String("user_id", userID),
		zap.String("role_id", roleID),
		zap.String("role_name", roleName),
	)

	role := gocloak.Role{
		ID:   &roleID,
		Name: &roleName,
	}

	if err := o.client.AddRealmRoleToUser(ctx, userID, role); err != nil {
		return nil, nil, fmt.Errorf("baton-keycloak: failed to add realm role to user: %w", err)
	}

	newGrant := grant.NewGrant(ent.Resource, "assigned", &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     userID,
		},
	})

	return []*v2.Grant{newGrant}, nil, nil
}

func (o *realmRoleBuilder) Revoke(ctx context.Context, g *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if g.Entitlement.Resource.Id.ResourceType != realmRoleResourceType.Id {
		return nil, fmt.Errorf("baton-keycloak: cannot revoke entitlement on non-realm-role resource")
	}

	roleID := g.Entitlement.Resource.Id.Resource
	roleName := g.Entitlement.Resource.DisplayName
	userID := g.Principal.Id.Resource

	l.Info("Revoking realm role from user",
		zap.String("user_id", userID),
		zap.String("role_id", roleID),
		zap.String("role_name", roleName),
	)

	role := gocloak.Role{
		ID:   &roleID,
		Name: &roleName,
	}

	if err := o.client.DeleteRealmRoleFromUser(ctx, userID, role); err != nil {
		return nil, fmt.Errorf("baton-keycloak: failed to remove realm role from user: %w", err)
	}

	return nil, nil
}

func parseIntoRealmRoleResource(role *gocloak.Role) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name":        safeString(role.Name),
		"description": safeString(role.Description),
	}

	if role.Composite != nil {
		profile["composite"] = *role.Composite
	}

	roleTraits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		safeString(role.Name),
		realmRoleResourceType,
		safeString(role.ID),
		roleTraits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newRealmRoleBuilder(client *client.Client) *realmRoleBuilder {
	return &realmRoleBuilder{
		resourceType: realmRoleResourceType,
		client:       client,
	}
}
