package connector

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/conductorone/baton-keycloak/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const minKeycloakVersionForSubGroups = 23

type Connector struct {
	client        *client.Client
	serverURL     string
	realm         string
	clientID      string
	clientSecret  string
	syncSubGroups bool
}

// ResourceSyncers returns ResourceSyncer for each resource type that should be synced from the upstream service.
func (c *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
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
	l := ctxzap.Extract(ctx)

	_, _, err := c.client.GetUsers(ctx, 0)
	if err != nil {
		return nil, err
	}

	// If syncSubGroups is enabled, validate Keycloak version
	if c.syncSubGroups {
		serverInfo, err := c.client.GetServerInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("sync-sub-groups is enabled but could not verify Keycloak version: %w", err)
		}

		if serverInfo.SystemInfo == nil || serverInfo.SystemInfo.Version == nil {
			return nil, fmt.Errorf("sync-sub-groups is enabled but could not determine Keycloak version")
		}

		version := *serverInfo.SystemInfo.Version
		majorVersion, err := parseMajorVersion(version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Keycloak version '%s': %w", version, err)
		}

		if majorVersion < minKeycloakVersionForSubGroups {
			return nil, fmt.Errorf("sync-sub-groups requires Keycloak version %d or newer, but server is running version %s", minKeycloakVersionForSubGroups, version)
		}

		l.Debug("Keycloak version validated for subgroup sync", zap.String("version", version))
	}

	return nil, nil
}

// parseMajorVersion extracts the major version number from a version string like "23.0.1" or "24.0.0-SNAPSHOT".
func parseMajorVersion(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid version format")
	}
	// Handle versions like "23" or "23.0.1" or "24.0.0-SNAPSHOT"
	majorStr := strings.Split(parts[0], "-")[0]
	return strconv.Atoi(majorStr)
}

func (c *Connector) Close() error {
	// Only close the Keycloak client connection
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Actually create a Keycloak connector.
func New(ctx context.Context, keycloakServerURL string, keycloakRealm string, keycloakClientID string, keycloakClientSecret string, syncSubGroups bool) (*Connector, error) {
	l := ctxzap.Extract(ctx)
	keycloakClient, err := client.NewClient(keycloakServerURL, keycloakRealm, keycloakClientID, keycloakClientSecret)
	if err != nil {
		l.Error("error creating Keycloak client for some reason", zap.Error(err))
		return nil, err
	}

	return &Connector{
		client:        keycloakClient,
		serverURL:     keycloakServerURL,
		realm:         keycloakRealm,
		clientID:      keycloakClientID,
		clientSecret:  keycloakClientSecret,
		syncSubGroups: syncSubGroups,
	}, nil
}
