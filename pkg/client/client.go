package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Clarilab/gocloaksession"
	"github.com/Nerzal/gocloak/v13"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
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

// AddUserToGroup adds a user to a group.
// PUT {{base_url}}/admin/realms/{realm}/users/{userId}/groups/{groupId}.
func (c *Client) AddUserToGroup(ctx context.Context, userID, groupID string) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.AddUserToGroup(ctx, token.AccessToken, c.realm, userID, groupID)
}

// RemoveUserFromGroup removes a user from a group.
// DELETE {{base_url}}/admin/realms/{realm}/users/{userId}/groups/{groupId}.
func (c *Client) RemoveUserFromGroup(ctx context.Context, userID, groupID string) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.DeleteUserFromGroup(ctx, token.AccessToken, c.realm, userID, groupID)
}

// CreateUser creates a user in the realm and returns the new user's ID.
// POST {{base_url}}/admin/realms/{realm}/users.
func (c *Client) CreateUser(ctx context.Context, user gocloak.User) (string, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	userID, err := c.client.CreateUser(ctx, token.AccessToken, c.realm, user)
	if err != nil {
		return "", uhttp.WrapErrors(MapAPIError(err), "failed to create user", err)
	}

	return userID, nil
}

// GetUserByID fetches a single user by ID.
// GET {{base_url}}/admin/realms/{realm}/users/{id}.
func (c *Client) GetUserByID(ctx context.Context, userID string) (*gocloak.User, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	user, err := c.client.GetUserByID(ctx, token.AccessToken, c.realm, userID)
	if err != nil {
		return nil, uhttp.WrapErrors(MapAPIError(err), "failed to get user by id", err)
	}

	return user, nil
}

// DeleteUser permanently removes a user.
// DELETE {{base_url}}/admin/realms/{realm}/users/{id}.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if err := c.client.DeleteUser(ctx, token.AccessToken, c.realm, userID); err != nil {
		return uhttp.WrapErrors(MapAPIError(err), "failed to delete user", err)
	}

	return nil
}

// SetUserEnabled toggles a user's enabled flag, reading the user first so the
// update preserves other fields and is skipped when already in that state.
// GET then PUT {{base_url}}/admin/realms/{realm}/users/{id}.
func (c *Client) SetUserEnabled(ctx context.Context, userID string, enabled bool) error {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	user, err := c.client.GetUserByID(ctx, token.AccessToken, c.realm, userID)
	if err != nil {
		return uhttp.WrapErrors(MapAPIError(err), "failed to get user by id", err)
	}

	if user.Enabled != nil && *user.Enabled == enabled {
		return nil
	}

	user.Enabled = pointer(enabled)
	if err := c.client.UpdateUser(ctx, token.AccessToken, c.realm, *user); err != nil {
		return uhttp.WrapErrors(MapAPIError(err), "failed to update user enabled state", err)
	}

	return nil
}

// UpdateUserProfile updates a user's email/firstName/lastName and returns the
// changed field names. The user is read first so the update preserves other
// fields; an empty result means profile had no updatable field (no write).
// GET then PUT {{base_url}}/admin/realms/{realm}/users/{id}.
func (c *Client) UpdateUserProfile(ctx context.Context, userID string, profile map[string]any) ([]string, error) {
	email, hasEmail := profileString(profile, "email")
	firstName, hasFirstName := profileString(profile, "firstName")
	lastName, hasLastName := profileString(profile, "lastName")
	if !hasEmail && !hasFirstName && !hasLastName {
		return nil, nil
	}

	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	user, err := c.client.GetUserByID(ctx, token.AccessToken, c.realm, userID)
	if err != nil {
		return nil, uhttp.WrapErrors(MapAPIError(err), "failed to get user by id", err)
	}

	var updated []string
	if hasEmail {
		user.Email = pointer(email)
		updated = append(updated, "email")
	}
	if hasFirstName {
		user.FirstName = pointer(firstName)
		updated = append(updated, "firstName")
	}
	if hasLastName {
		user.LastName = pointer(lastName)
		updated = append(updated, "lastName")
	}

	if err := c.client.UpdateUser(ctx, token.AccessToken, c.realm, *user); err != nil {
		return nil, uhttp.WrapErrors(MapAPIError(err), "failed to update user profile", err)
	}

	return updated, nil
}

// profileString returns the string value at key and whether it was present as a string.
func profileString(profile map[string]any, key string) (string, bool) {
	v, ok := profile[key].(string)
	return v, ok
}

// GetUserByUsername returns the user matching username exactly, or nil when none exists.
// GET {{base_url}}/admin/realms/{realm}/users?username={username}&exact=true.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*gocloak.User, error) {
	return c.getUserByExactParams(ctx, gocloak.GetUsersParams{
		Username: pointer(username),
		Exact:    pointer(true),
		Max:      pointer(1),
	}, "username")
}

// GetUserByEmail returns the user matching email exactly, or nil when none exists.
// GET {{base_url}}/admin/realms/{realm}/users?email={email}&exact=true.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*gocloak.User, error) {
	return c.getUserByExactParams(ctx, gocloak.GetUsersParams{
		Email: pointer(email),
		Exact: pointer(true),
		Max:   pointer(1),
	}, "email")
}

// getUserByExactParams runs an exact-match user lookup and returns the single
// result, or nil when none exists; field names the lookup key for the error message.
// GET {{base_url}}/admin/realms/{realm}/users.
func (c *Client) getUserByExactParams(ctx context.Context, params gocloak.GetUsersParams, field string) (*gocloak.User, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	users, err := c.client.GetUsers(ctx, token.AccessToken, c.realm, params)
	if err != nil {
		return nil, uhttp.WrapErrors(MapAPIError(err), fmt.Sprintf("failed to get user by %s", field), err)
	}

	if len(users) == 0 {
		return nil, nil
	}

	return users[0], nil
}

// GetUsers returns a page of realm users.
// GET {{base_url}}/admin/realms/{realm}/users.
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

// GetGroupMembers returns a page of a group's members.
// GET {{base_url}}/admin/realms/{realm}/groups/{groupId}/members.
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

// GetGroups returns a page of top-level realm groups.
// GET {{base_url}}/admin/realms/{realm}/groups.
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

// GetUserGroups returns the groups a user belongs to.
// GET {{base_url}}/admin/realms/{realm}/users/{userId}/groups.
func (c *Client) GetUserGroups(ctx context.Context, userID string) ([]*gocloak.Group, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.GetUserGroups(ctx, token.AccessToken, c.realm, userID, gocloak.GetGroupsParams{})
}

// GetGroupChildren returns a page of a group's child (sub)groups.
// GET {{base_url}}/admin/realms/{realm}/groups/{groupId}/children.
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

// GetKeycloakVersion returns the server's major version.
// GET {{base_url}}/admin/serverinfo.
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
