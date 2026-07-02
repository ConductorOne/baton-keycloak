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

// CreateUser creates a user in the realm via POST /admin/realms/{realm}/users and returns the new user's ID.
func (c *Client) CreateUser(ctx context.Context, user gocloak.User) (string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	userID, err := c.client.CreateUser(ctx, token.AccessToken, c.realm, user)
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return userID, nil
}

// GetUserByID fetches a single user by ID via GET /admin/realms/{realm}/users/{id}.
func (c *Client) GetUserByID(ctx context.Context, userID string) (*gocloak.User, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	user, err := c.client.GetUserByID(ctx, token.AccessToken, c.realm, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return user, nil
}

// GetUserByUsername returns the user matching username exactly, or nil when none exists.
// Used by CreateAccount to read back a pre-existing user after a 409 conflict.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*gocloak.User, error) {
	return c.getUserByExactParams(ctx, gocloak.GetUsersParams{
		Username: pointer(username),
		Exact:    pointer(true),
		Max:      pointer(1),
	}, "username")
}

// GetUserByEmail returns the user matching email exactly, or nil when none exists.
// Used as a fallback read-back when a create conflict (409) was triggered by a
// duplicate email rather than a duplicate username.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*gocloak.User, error) {
	return c.getUserByExactParams(ctx, gocloak.GetUsersParams{
		Email: pointer(email),
		Exact: pointer(true),
		Max:   pointer(1),
	}, "email")
}

// getUserByExactParams runs an exact-match user lookup (by username or email) and
// returns the single result, or nil when none exists. field names the lookup key
// for the error message. Shared by GetUserByUsername and GetUserByEmail.
func (c *Client) getUserByExactParams(ctx context.Context, params gocloak.GetUsersParams, field string) (*gocloak.User, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	users, err := c.client.GetUsers(ctx, token.AccessToken, c.realm, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by %s: %w", field, err)
	}

	if len(users) == 0 {
		return nil, nil
	}

	return users[0], nil
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

func (c *Client) Close() error {
	return nil
}

func pointer[T any](v T) *T {
	return &v
}
