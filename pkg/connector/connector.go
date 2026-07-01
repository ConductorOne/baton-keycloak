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
	client          *client.Client
	serverURL       string
	realm           string
	clientID        string
	clientSecret    string
	syncSubGroups   bool
	keycloakVersion int
}

// ResourceSyncers returns ResourceSyncer for each resource type that should be synced from the upstream service.
func (c *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		newUserBuilder(c.client),
		newGroupBuilder(c.client, c.syncSubGroups, c.keycloakVersion),
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
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"username": {
					DisplayName: "Username",
					Required:    true,
					Description: "Username for the new Keycloak user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "john.doe",
					Order:       1,
				},
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "Email address for the new user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "john.doe@example.com",
					Order:       2,
				},
				"firstName": {
					DisplayName: "First Name",
					Required:    false,
					Description: "Given name (first name) of the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "John",
					Order:       3,
				},
				"lastName": {
					DisplayName: "Last Name",
					Required:    false,
					Description: "Family name (last name) of the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Doe",
					Order:       4,
				},
			},
		},
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

	// Use base-url override if provided, otherwise use keycloak-server-url
	serverURL := config.KeycloakServerUrl
	if config.BaseUrl != "" {
		serverURL = config.BaseUrl
	}

	keycloakClient, err := client.NewClient(
		serverURL,
		config.KeycloakRealm,
		config.KeycloakClientId,
		config.KeycloakClientSecret,
	)
	if err != nil {
		l.Error("error creating Keycloak client for some reason", zap.Error(err))
		return nil, nil, err
	}

	// Check Keycloak version once during initialization
	version, err := keycloakClient.GetKeycloakVersion(ctx)
	if err != nil {
		l.Debug("failed to get Keycloak version, defaulting to legacy behavior", zap.Error(err))
		version = 0 // Default to 0 (will use legacy behavior)
	} else {
		l.Debug("detected Keycloak version", zap.Int("major_version", version))
	}

	return &Connector{
		client:          keycloakClient,
		serverURL:       serverURL,
		realm:           config.KeycloakRealm,
		clientID:        config.KeycloakClientId,
		clientSecret:    config.KeycloakClientSecret,
		syncSubGroups:   config.SyncSubGroups,
		keycloakVersion: version,
	}, nil, nil
}
