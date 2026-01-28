package connector

import (
	"context"
	"io"

	"github.com/conductorone/baton-keycloak/pkg/client"
	cfg "github.com/conductorone/baton-keycloak/pkg/config"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type Connector struct {
	client        *client.Client
	serverURL     string
	realm         string
	clientID      string
	clientSecret  string
	syncSubGroups bool
}

// ResourceSyncers returns ResourceSyncer for each resource type that should be synced from the upstream service.
func (c *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		newUserBuilder(c.client),
		newGroupBuilder(c.client, c.syncSubGroups),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client
// crossing my fingers that this is not needed tbh.
func (c *Connector) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector for C1 in the logs and whatnot. It will also display in the UI. Sadly emojis are not supported.
func (c *Connector) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "baton-keycloak",
		Description: "Connector syncing users and groups from Keycloak",
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should test API credentials.
func (c *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, _, err := c.client.GetUsers(ctx, 0)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *Connector) Close() error {
	// Only close the Keycloak client connection
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func New(ctx context.Context, config *cfg.Keycloak, opts *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	l := ctxzap.Extract(ctx)
	keycloakClient, err := client.NewClient(
		config.KeycloakServerUrl,
		config.KeycloakRealm,
		config.KeycloakClientId,
		config.KeycloakClientSecret,
	)
	if err != nil {
		l.Error("error creating Keycloak client for some reason", zap.Error(err))
		return nil, nil, err
	}

	return &Connector{
		client:        keycloakClient,
		serverURL:     config.KeycloakServerUrl,
		realm:         config.KeycloakRealm,
		clientID:      config.KeycloakClientId,
		clientSecret:  config.KeycloakClientSecret,
		syncSubGroups: config.SyncSubGroups,
	}, nil, nil
}
