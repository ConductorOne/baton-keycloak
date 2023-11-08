package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

const userUrl = "/admin/realms/master/users"
const groupsUrl = "/admin/realms/master/groups"
const membersUrl = "/admin/realms/master/groups/{id}/members"
const refreshUrl = "/realms/master/protocol/openid-connect/token"

type Client struct {
	accessToken string
	httpClient  *http.Client
	baseUrl     string
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
}

type User struct {
	Id                         string        `json:"id"`
	CreatedTimestamp           int64         `json:"createdTimestamp"`
	Username                   string        `json:"username"`
	Enabled                    bool          `json:"enabled"`
	Totp                       bool          `json:"totp"`
	EmailVerified              bool          `json:"emailVerified"`
	DisableableCredentialTypes []interface{} `json:"disableableCredentialTypes"`
	RequiredActions            []interface{} `json:"requiredActions"`
	NotBefore                  int           `json:"notBefore"`
	Access                     struct {
		ManageGroupMembership bool `json:"manageGroupMembership"`
		View                  bool `json:"view"`
		MapRoles              bool `json:"mapRoles"`
		Impersonate           bool `json:"impersonate"`
		Manage                bool `json:"manage"`
	} `json:"access"`
}

type Group struct {
	Id        string        `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	SubGroups []interface{} `json:"subGroups"`
}

func isExpired(accessToken string) (bool, error) {
	fmt.Printf("Is Expired- access token %s \n", accessToken)
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("<YOUR VERIFICATION KEY>"), nil
	})

	if err != nil {
		fmt.Printf("Error: %s \n", err.Error())
		//return true, fmt.Errorf("unable to decode access token")
	}

	exp, ok := claims["exp"]
	if !ok {
		return true, fmt.Errorf("malformed access token")
	}

	expiry, ok := exp.(float64)
	if !ok {
		return true, fmt.Errorf("unable to convert expiration to float64")
	}

	expTimestamp := time.Unix(int64(expiry), 0)
	currentTime := time.Now()

	fmt.Printf("Expired timestamp: %s, Current timestamp: %s", expTimestamp.String(), currentTime.String())
	if expTimestamp.Before(currentTime) {
		return true, nil
	}

	return false, nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	isExpired, err := isExpired(c.accessToken)
	if err != nil {
		return err
	}
	if isExpired {
		fmt.Printf("Access token is expired \n")
		var token AccessToken
		//response, err := http.PostForm("http://"+path.Join(c.baseUrl, refreshUrl), url.Values{"username": {"keshav"}, "password": {"c1test12345"}, "client-id": {"admin-cli"}, "grant_type": {"password"}})
		request, err := http.NewRequest(http.MethodPost, "http://"+path.Join(c.baseUrl, refreshUrl), strings.NewReader(`username=keshav&password=c1test12345&client_id=admin-cli&grant_type=password`))
		if err != nil {
			fmt.Printf("Http request failed \n")
			return err
		}

		request.Header.Add("Content-type", "application/x-www-form-urlencoded")

		response, err := c.httpClient.Do(request.WithContext(ctx))
		if err != nil {
			return err
		}
		defer response.Body.Close()

		ret, err := io.ReadAll(response.Body)

		if err != nil {
			return err
		}

		err = json.Unmarshal(ret, &token)
		if err != nil {
			return err
		}
		c.accessToken = token.AccessToken
		return nil
	}
	return nil
}

func New(accessToken string, baseUrl string) *Client {
	return &Client{
		accessToken: accessToken,
		baseUrl:     baseUrl,
		httpClient:  &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) ListUsers(ctx context.Context) ([]*User, error) {
	var ret []*User

	req, err := http.NewRequest(http.MethodGet, "http://"+path.Join(c.baseUrl, userUrl), nil)
	if err != nil {
		return nil, err
	}

	userBytes, err := c.do(ctx, req)

	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(userBytes).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Client) ListGroups(ctx context.Context) ([]*Group, error) {
	var ret []*Group

	req, err := http.NewRequest(http.MethodGet, "http://"+path.Join(c.baseUrl, groupsUrl), nil)
	if err != nil {
		return nil, err
	}

	groupBytes, err := c.do(ctx, req)

	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(groupBytes).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Client) GetGroup(ctx context.Context, groupId string) (*Group, error) {
	var ret *Group

	req, err := http.NewRequest(http.MethodGet, "http://"+path.Join(c.baseUrl, groupsUrl, groupId), nil)
	if err != nil {
		return nil, err
	}

	groupBytes, err := c.do(ctx, req)

	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(groupBytes).Decode(&ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *Client) ListGroupMembers(ctx context.Context, groupId string) ([]*User, error) {
	var ret []*User

	req, err := http.NewRequest(http.MethodGet, "http://"+path.Join(c.baseUrl, fmt.Sprintf(membersUrl, groupId)), nil)
	if err != nil {
		return nil, err
	}

	userBytes, err := c.do(ctx, req)

	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(userBytes).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Client) do(ctx context.Context, req *http.Request) (io.Reader, error) {
	err := c.refreshToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.accessToken)
	response, err := c.httpClient.Do(req.WithContext(ctx))

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	ret, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	fmt.Printf("%s \n", ret)

	return bytes.NewBuffer(ret), nil
}
