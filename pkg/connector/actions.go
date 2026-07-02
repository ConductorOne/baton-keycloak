package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-keycloak/pkg/client"
	config "github.com/conductorone/baton-sdk/pb/c1/config/v1"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/actions"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	actionDisableUser = "disable_user"
	actionEnableUser  = "enable_user"

	argUserIDKey  = "user_id"
	retSuccessKey = "success"
)

// successReturnType is the shared {"success": bool} return schema for every action.
var successReturnType = []*config.Field{
	{Name: retSuccessKey, DisplayName: "Success", Field: &config.Field_BoolField{}},
}

var disableUserSchema = &v2.BatonActionSchema{
	Name:        actionDisableUser,
	DisplayName: "Disable User",
	Description: "Deactivates a Keycloak user by setting enabled=false (reversible).",
	Arguments: []*config.Field{
		{Name: argUserIDKey, DisplayName: "User Resource ID", Description: "The ID of the Keycloak user to disable.", Field: &config.Field_StringField{}, IsRequired: true},
	},
	ReturnTypes: successReturnType,
	ActionType: []v2.ActionType{
		v2.ActionType_ACTION_TYPE_ACCOUNT,
		v2.ActionType_ACTION_TYPE_ACCOUNT_DISABLE,
	},
}

var enableUserSchema = &v2.BatonActionSchema{
	Name:        actionEnableUser,
	DisplayName: "Enable User",
	Description: "Reactivates a Keycloak user by setting enabled=true.",
	Arguments: []*config.Field{
		{Name: argUserIDKey, DisplayName: "User Resource ID", Description: "The ID of the Keycloak user to enable.", Field: &config.Field_StringField{}, IsRequired: true},
	},
	ReturnTypes: successReturnType,
	ActionType: []v2.ActionType{
		v2.ActionType_ACTION_TYPE_ACCOUNT,
		v2.ActionType_ACTION_TYPE_ACCOUNT_ENABLE,
	},
}

// Compile-time assertion: Connector must satisfy GlobalActionProvider so a
// mis-wired interface fails the build, not the platform. Global (not
// resource-scoped) registration is required so the actions are invokable via
// the ActionService (CLI --invoke-action and C1's "Perform connector action").
var _ connectorbuilder.GlobalActionProvider = (*Connector)(nil)

// GlobalActions registers the enable_user / disable_user lifecycle actions.
func (c *Connector) GlobalActions(ctx context.Context, registry actions.ActionRegistry) error {
	if err := registry.Register(ctx, disableUserSchema, c.disableUserHandler); err != nil {
		return fmt.Errorf("baton-keycloak: register disable_user: %w", err)
	}
	return registry.Register(ctx, enableUserSchema, c.enableUserHandler)
}

// disableUserHandler deactivates a user (enabled=false). Idempotent: Keycloak's
// update returns 204 whether or not the user was already disabled.
func (c *Connector) disableUserHandler(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
	userID, err := userIDArg(args)
	if err != nil {
		return nil, nil, err
	}

	if err := c.client.SetUserEnabled(ctx, userID, false); err != nil {
		if client.IsNotFoundError(err) {
			return nil, nil, status.Errorf(codes.NotFound, "baton-keycloak: disable_user: user %s not found", userID)
		}
		return nil, nil, status.Errorf(codes.Internal, "baton-keycloak: disable_user %s: %v", userID, err)
	}

	return successStruct(), nil, nil
}

// enableUserHandler reactivates a user (enabled=true). Idempotent like disable.
func (c *Connector) enableUserHandler(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
	userID, err := userIDArg(args)
	if err != nil {
		return nil, nil, err
	}

	if err := c.client.SetUserEnabled(ctx, userID, true); err != nil {
		if client.IsNotFoundError(err) {
			return nil, nil, status.Errorf(codes.NotFound, "baton-keycloak: enable_user: user %s not found", userID)
		}
		return nil, nil, status.Errorf(codes.Internal, "baton-keycloak: enable_user %s: %v", userID, err)
	}

	return successStruct(), nil, nil
}

// userIDArg extracts and validates the required user_id action argument nil-safely.
func userIDArg(args *structpb.Struct) (string, error) {
	if args == nil {
		return "", status.Error(codes.InvalidArgument, "baton-keycloak: missing arguments")
	}
	userID, ok := actions.GetStringArg(args, argUserIDKey)
	if !ok || userID == "" {
		return "", status.Error(codes.InvalidArgument, "baton-keycloak: user_id is required")
	}
	return userID, nil
}

// successStruct returns the standard {"success": true} action response. Direct
// construction avoids the always-nil error from structpb.NewStruct for a bool.
func successStruct() *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			retSuccessKey: {Kind: &structpb.Value_BoolValue{BoolValue: true}},
		},
	}
}
