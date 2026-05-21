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

type clientRoleBuilder struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

func (o *clientRoleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return clientRoleResourceType
}

func (o *clientRoleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, nil, nil
	}

	if parentResourceID.ResourceType != clientResourceType.Id {
		return nil, nil, nil
	}

	clientID := parentResourceID.Resource

	roles, nextToken, err := o.client.GetClientRoles(ctx, clientID, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	var resources []*v2.Resource
	for _, role := range roles {
		resource, err := parseIntoClientRoleResource(role, parentResourceID)
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

func (o *clientRoleBuilder) Entitlements(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	assignmentEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"assigned",
		entitlement.WithDisplayName(fmt.Sprintf("Assigned %s", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Assigned the %s client role", resource.DisplayName)),
		entitlement.WithGrantableTo(userResourceType),
	)

	return []*v2.Entitlement{assignmentEntitlement}, nil, nil
}

func (o *clientRoleBuilder) Grants(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	parentResourceID := resource.GetParentResourceId()
	if parentResourceID == nil {
		return nil, nil, fmt.Errorf("baton-keycloak: client role missing parent client resource")
	}

	clientID := parentResourceID.Resource
	roleName := resource.DisplayName

	users, nextToken, err := o.client.GetUsersByClientRoleName(ctx, clientID, roleName, parseToken(&attrs.PageToken))
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

func (o *clientRoleBuilder) Grant(ctx context.Context, resource *v2.Resource, ent *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if resource.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("baton-keycloak: cannot grant client role to non-user resource")
	}

	parentResourceID := ent.Resource.GetParentResourceId()
	if parentResourceID == nil {
		return nil, nil, fmt.Errorf("baton-keycloak: client role entitlement missing parent client resource")
	}

	clientID := parentResourceID.Resource
	roleID := ent.Resource.Id.Resource
	roleName := ent.Resource.DisplayName
	userID := resource.Id.Resource

	l.Info("Granting client role to user",
		zap.String("user_id", userID),
		zap.String("client_id", clientID),
		zap.String("role_id", roleID),
		zap.String("role_name", roleName),
	)

	role := gocloak.Role{
		ID:   &roleID,
		Name: &roleName,
	}

	if err := o.client.AddClientRoleToUser(ctx, clientID, userID, role); err != nil {
		return nil, nil, fmt.Errorf("baton-keycloak: failed to add client role to user: %w", err)
	}

	newGrant := grant.NewGrant(ent.Resource, "assigned", &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     userID,
		},
	})

	return []*v2.Grant{newGrant}, nil, nil
}

func (o *clientRoleBuilder) Revoke(ctx context.Context, g *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if g.Entitlement.Resource.Id.ResourceType != clientRoleResourceType.Id {
		return nil, fmt.Errorf("baton-keycloak: cannot revoke entitlement on non-client-role resource")
	}

	parentResourceID := g.Entitlement.Resource.GetParentResourceId()
	if parentResourceID == nil {
		return nil, fmt.Errorf("baton-keycloak: client role grant missing parent client resource")
	}

	clientID := parentResourceID.Resource
	roleID := g.Entitlement.Resource.Id.Resource
	roleName := g.Entitlement.Resource.DisplayName
	userID := g.Principal.Id.Resource

	l.Info("Revoking client role from user",
		zap.String("user_id", userID),
		zap.String("client_id", clientID),
		zap.String("role_id", roleID),
		zap.String("role_name", roleName),
	)

	role := gocloak.Role{
		ID:   &roleID,
		Name: &roleName,
	}

	if err := o.client.DeleteClientRoleFromUser(ctx, clientID, userID, role); err != nil {
		return nil, fmt.Errorf("baton-keycloak: failed to remove client role from user: %w", err)
	}

	return nil, nil
}

func parseIntoClientRoleResource(role *gocloak.Role, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
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
		clientRoleResourceType,
		safeString(role.ID),
		roleTraits,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newClientRoleBuilder(client *client.Client) *clientRoleBuilder {
	return &clientRoleBuilder{
		resourceType: clientRoleResourceType,
		client:       client,
	}
}
