package connector

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

type clientBuilder struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

func (o *clientBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return clientResourceType
}

func (o *clientBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	clients, nextToken, err := o.client.GetClients(ctx, parseToken(&attrs.PageToken))
	if err != nil {
		return nil, nil, err
	}

	var resources []*v2.Resource
	for _, c := range clients {
		resource, err := parseIntoClientResource(c)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, resource)
	}

	if len(clients) == 0 {
		nextToken = ""
	}

	return resources, &rs.SyncOpResults{NextPageToken: nextToken}, nil
}

func (o *clientBuilder) Entitlements(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *clientBuilder) Grants(ctx context.Context, resource *v2.Resource, attrs rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func parseIntoClientResource(c *gocloak.Client) (*v2.Resource, error) {
	displayName := safeString(c.ClientID)
	if name := safeString(c.Name); name != "" {
		displayName = name
	}

	profile := map[string]interface{}{
		"client_id": safeString(c.ClientID),
		"name":      safeString(c.Name),
		"protocol":  safeString(c.Protocol),
	}

	if c.Description != nil {
		profile["description"] = *c.Description
	}

	if c.Enabled != nil {
		profile["enabled"] = *c.Enabled
	}

	appTraits := []rs.AppTraitOption{
		rs.WithAppProfile(profile),
	}

	ret, err := rs.NewAppResource(
		displayName,
		clientResourceType,
		safeString(c.ID),
		appTraits,
		rs.WithAnnotation(&v2.ChildResourceType{
			ResourceTypeId: clientRoleResourceType.Id,
		}),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newClientBuilder(client *client.Client) *clientBuilder {
	return &clientBuilder{
		resourceType: clientResourceType,
		client:       client,
	}
}
