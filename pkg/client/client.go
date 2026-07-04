package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Clarilab/gocloaksession"
	"github.com/Nerzal/gocloak/v13"
)

type Client struct {
	client       *gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string
	session      gocloaksession.GoCloakSession
	serverURL    string
}

var defaultMax = pointer(100)

func NewClient(serverURL, realm, clientID, clientSecret string) (*Client, error) {
	session, err := gocloaksession.NewSession(clientID, clientSecret, realm, serverURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:       gocloak.NewClient(serverURL),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		session:      session,
		serverURL:    serverURL,
	}, nil
}

func (c *Client) AddUserToGroup(ctx context.Context, userID, groupID string) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.AddUserToGroup(ctx, token.AccessToken, c.realm, userID, groupID)
}

func (c *Client) RemoveUserFromGroup(ctx context.Context, userID, groupID string) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.DeleteUserFromGroup(ctx, token.AccessToken, c.realm, userID, groupID)
}

func (c *Client) GetUsers(ctx context.Context, first int) ([]*gocloak.User, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get token: %w", err)
	}
	users, err := c.client.GetUsers(ctx, token.AccessToken, c.realm, gocloak.GetUsersParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(users) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(users))
	}

	return users, nextToken, nil
}

func (c *Client) GetGroupMembers(ctx context.Context, groupID string, first int) ([]*gocloak.User, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	members, err := c.client.GetGroupMembers(ctx, token.AccessToken, c.realm, groupID, gocloak.GetGroupsParams{
		First: pointer(first),
		Max:   defaultMax,
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to get group members: %w", err)
	}

	if len(members) == 0 {
		return nil, "", nil
	}

	// If we got fewer items than requested, we've reached the last page
	nextToken := ""
	if len(members) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(members))
	}

	return members, nextToken, nil
}

func (c *Client) GetGroups(ctx context.Context, first int) ([]*Group, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get token: %w", err)
	}

	u, err := url.Parse(c.serverURL)
	if err != nil {
		return nil, strconv.Itoa(first), err
	}
	u = u.JoinPath("admin", "realms", c.realm, "groups")

	var result []*Group
	resp, err := c.client.GetRequestWithBearerAuth(ctx, token.AccessToken).
		SetResult(&result).
		SetQueryParams(map[string]string{
			"first":               strconv.Itoa(first),
			"max":                 strconv.Itoa(*defaultMax),
			"subGroupsCount":      "true",
			"briefRepresentation": "false",
		}).
		Get(u.String())

	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get groups: %w", err)
	}

	if resp.IsError() {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get groups: status %s", resp.Status())
	}

	if len(result) == 0 {
		return nil, "", nil
	}

	// If we got fewer items than requested, we've reached the last page
	nextToken := ""
	if len(result) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(result))
	}

	return result, nextToken, nil
}

func (c *Client) GetUserGroups(ctx context.Context, userID string) ([]*gocloak.Group, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.GetUserGroups(ctx, token.AccessToken, c.realm, userID, gocloak.GetGroupsParams{})
}

func (c *Client) GetGroupChildren(ctx context.Context, groupID string, first int) ([]*Group, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get token: %w", err)
	}

	u, err := url.Parse(c.serverURL)
	if err != nil {
		return nil, strconv.Itoa(first), err
	}
	u = u.JoinPath("admin", "realms", c.realm, "groups", groupID, "children")

	var result []*Group
	resp, err := c.client.GetRequestWithBearerAuth(ctx, token.AccessToken).
		SetResult(&result).
		SetQueryParams(map[string]string{
			"first":               strconv.Itoa(first),
			"max":                 strconv.Itoa(*defaultMax),
			"briefRepresentation": "false",
		}).
		Get(u.String())

	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get group children: %w", err)
	}

	if resp.IsError() {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get group children: status %s", resp.Status())
	}

	if len(result) == 0 {
		return nil, "", nil
	}

	// If we got fewer items than requested, we've reached the last page
	nextToken := ""
	if len(result) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(result))
	}

	return result, nextToken, nil
}

func (c *Client) GetKeycloakVersion(ctx context.Context) (int, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return 0, fmt.Errorf("failed to get token: %w", err)
	}

	serverInfo, err := c.client.GetServerInfo(ctx, token.AccessToken)
	if err != nil {
		return 0, fmt.Errorf("failed to get server info: %w", err)
	}

	if serverInfo.SystemInfo == nil || serverInfo.SystemInfo.Version == nil {
		return 0, fmt.Errorf("server version not available")
	}

	version := *serverInfo.SystemInfo.Version
	// Parse version string like "23.0.1" or "24.0.0-SNAPSHOT" to get major version
	majorVersion, err := parseMajorVersion(version)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version '%s': %w", version, err)
	}

	return majorVersion, nil
}

// parseMajorVersion extracts the major version number from a version string.
func parseMajorVersion(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid version format")
	}
	// Handle versions like "23" or "23.0.1" or "24.0.0-SNAPSHOT"
	majorStr := strings.Split(parts[0], "-")[0]
	return strconv.Atoi(majorStr)
}

func (c *Client) GetRealmRoles(ctx context.Context, first int) ([]*gocloak.Role, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	roles, err := c.client.GetRealmRoles(ctx, token.AccessToken, c.realm, gocloak.GetRoleParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get realm roles: %w", err)
	}

	if len(roles) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(roles) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(roles))
	}

	return roles, nextToken, nil
}

func (c *Client) GetUsersByRealmRoleName(ctx context.Context, roleName string, first int) ([]*gocloak.User, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	users, err := c.client.GetUsersByRoleName(ctx, token.AccessToken, c.realm, roleName, gocloak.GetUsersByRoleParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get users by realm role name: %w", err)
	}

	if len(users) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(users) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(users))
	}

	return users, nextToken, nil
}

func (c *Client) AddRealmRoleToUser(ctx context.Context, userID string, role gocloak.Role) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.AddRealmRoleToUser(ctx, token.AccessToken, c.realm, userID, []gocloak.Role{role})
}

func (c *Client) DeleteRealmRoleFromUser(ctx context.Context, userID string, role gocloak.Role) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.DeleteRealmRoleFromUser(ctx, token.AccessToken, c.realm, userID, []gocloak.Role{role})
}

func (c *Client) GetClients(ctx context.Context, first int) ([]*gocloak.Client, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	clients, err := c.client.GetClients(ctx, token.AccessToken, c.realm, gocloak.GetClientsParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get clients: %w", err)
	}

	if len(clients) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(clients) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(clients))
	}

	return clients, nextToken, nil
}

func (c *Client) GetClientRoles(ctx context.Context, idOfClient string, first int) ([]*gocloak.Role, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	roles, err := c.client.GetClientRoles(ctx, token.AccessToken, c.realm, idOfClient, gocloak.GetRoleParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get client roles: %w", err)
	}

	if len(roles) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(roles) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(roles))
	}

	return roles, nextToken, nil
}

func (c *Client) GetUsersByClientRoleName(ctx context.Context, idOfClient, roleName string, first int) ([]*gocloak.User, string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	users, err := c.client.GetUsersByClientRoleName(ctx, token.AccessToken, c.realm, idOfClient, roleName, gocloak.GetUsersByRoleParams{
		First: pointer(first),
		Max:   defaultMax,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get users by client role name: %w", err)
	}

	if len(users) == 0 {
		return nil, "", nil
	}

	nextToken := ""
	if len(users) >= *defaultMax {
		nextToken = strconv.Itoa(first + len(users))
	}

	return users, nextToken, nil
}

func (c *Client) AddClientRoleToUser(ctx context.Context, idOfClient, userID string, role gocloak.Role) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.AddClientRolesToUser(ctx, token.AccessToken, c.realm, idOfClient, userID, []gocloak.Role{role})
}

func (c *Client) DeleteClientRoleFromUser(ctx context.Context, idOfClient, userID string, role gocloak.Role) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.DeleteClientRolesFromUser(ctx, token.AccessToken, c.realm, idOfClient, userID, []gocloak.Role{role})
}

func (c *Client) Close() error {
	return nil
}

func pointer[T any](v T) *T {
	return &v
}
