package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

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

	if len(users) < *defaultMax {
		return users, "", nil
	}

	return users, strconv.Itoa(first + *defaultMax), nil
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

	if len(members) < *defaultMax {
		return members, "", nil
	}

	return members, strconv.Itoa(first + len(members)), nil
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
			"briefRepresentation": "false",
		}).
		Get(u.String())

	if err != nil {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get groups: %w", err)
	}

	if resp.IsError() {
		return nil, strconv.Itoa(first), fmt.Errorf("failed to get groups: status %s", resp.Status())
	}

	if len(result) < *defaultMax {
		return result, "", nil
	}

	return result, strconv.Itoa(first + len(result)), nil
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

	if len(result) < *defaultMax {
		return result, "", nil
	}

	return result, strconv.Itoa(first + len(result)), nil
}

func (c *Client) GetUserGroups(ctx context.Context, userID string) ([]*gocloak.Group, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.GetUserGroups(ctx, token.AccessToken, c.realm, userID, gocloak.GetGroupsParams{})
}

// GetServerInfo returns the Keycloak server info including version.
func (c *Client) GetServerInfo(ctx context.Context) (*gocloak.ServerInfoRepresentation, error) {
	token, err := c.session.GetKeycloakAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return c.client.GetServerInfo(ctx, token.AccessToken)
}

func (c *Client) Close() error {
	return nil
}

func pointer[T any](v T) *T {
	return &v
}
