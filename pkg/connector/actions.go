package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	actionUpdateUser  = "update_user"

	argUserIDKey      = "user_id"
	argUserIDDisplay  = "User Resource ID"
	argUserProfileKey = "user_profile"

	retSuccessKey       = "success"
	retUpdatedFieldsKey = "updated_fields"
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
		{Name: argUserIDKey, DisplayName: argUserIDDisplay, Description: "The ID of the Keycloak user to disable.", Field: &config.Field_StringField{}, IsRequired: true},
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
		{Name: argUserIDKey, DisplayName: argUserIDDisplay, Description: "The ID of the Keycloak user to enable.", Field: &config.Field_StringField{}, IsRequired: true},
	},
	ReturnTypes: successReturnType,
	ActionType: []v2.ActionType{
		v2.ActionType_ACTION_TYPE_ACCOUNT,
		v2.ActionType_ACTION_TYPE_ACCOUNT_ENABLE,
	},
}

// updatedFieldsReturnType is the {"success": bool, "updated_fields": string} return
// schema for update_user, reporting which profile fields the call changed.
var updatedFieldsReturnType = []*config.Field{
	{Name: retSuccessKey, DisplayName: "Success", Field: &config.Field_BoolField{}},
	{Name: retUpdatedFieldsKey, DisplayName: "Updated Fields", Field: &config.Field_StringField{}},
}

var updateUserSchema = &v2.BatonActionSchema{
	Name:        actionUpdateUser,
	DisplayName: "Update User",
	Description: "Updates a Keycloak user's profile attributes (email, firstName, lastName) from a user_profile JSON object.",
	Arguments: []*config.Field{
		{Name: argUserIDKey, DisplayName: argUserIDDisplay, Description: "The ID of the Keycloak user to update.", Field: &config.Field_StringField{}, IsRequired: true},
		{Name: argUserProfileKey, DisplayName: "User Profile Data", Description: "Attributes to update: email, firstName, lastName.", Field: &config.Field_StringField{}, IsRequired: true},
	},
	ReturnTypes: updatedFieldsReturnType,
	ActionType: []v2.ActionType{
		v2.ActionType_ACTION_TYPE_ACCOUNT,
		v2.ActionType_ACTION_TYPE_ACCOUNT_UPDATE_PROFILE,
	},
}

// Compile-time check that Connector implements GlobalActionProvider.
var _ connectorbuilder.GlobalActionProvider = (*Connector)(nil)

// GlobalActions registers the enable_user / disable_user lifecycle actions and the
// update_user profile-update action.
func (c *Connector) GlobalActions(ctx context.Context, registry actions.ActionRegistry) error {
	if err := registry.Register(ctx, disableUserSchema, c.setEnabledHandler(actionDisableUser, false)); err != nil {
		return fmt.Errorf("baton-keycloak: register disable_user: %w", err)
	}
	if err := registry.Register(ctx, enableUserSchema, c.setEnabledHandler(actionEnableUser, true)); err != nil {
		return fmt.Errorf("baton-keycloak: register enable_user: %w", err)
	}
	if err := registry.Register(ctx, updateUserSchema, c.updateUserHandler); err != nil {
		return fmt.Errorf("baton-keycloak: register update_user: %w", err)
	}
	return nil
}

// updateUserHandler updates a user's profile attributes from the user_profile
// argument. C1 sends user_profile as a JSON string (push rules) or a struct
// (manual invocation); both are handled. A request with no updatable field is
// rejected as InvalidArgument, and a missing user maps to NotFound.
func (c *Connector) updateUserHandler(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
	userID, err := userIDArg(args)
	if err != nil {
		return nil, nil, err
	}

	profile, err := profileArgAsMap(args, argUserProfileKey)
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-keycloak: %s: %v", actionUpdateUser, err)
	}

	updated, err := c.client.UpdateUserProfile(ctx, userID, profile)
	if err != nil {
		if client.IsNotFoundError(err) {
			return nil, nil, status.Errorf(codes.NotFound, "baton-keycloak: %s: user %s not found", actionUpdateUser, userID)
		}
		return nil, nil, fmt.Errorf("baton-keycloak: %s user %s: %w", actionUpdateUser, userID, err)
	}
	if len(updated) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-keycloak: %s: user_profile must set at least one of email, firstName, lastName", actionUpdateUser)
	}

	result, err := structpb.NewStruct(map[string]any{
		retSuccessKey:       true,
		retUpdatedFieldsKey: strings.Join(updated, ", "),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("baton-keycloak: %s: build result: %w", actionUpdateUser, err)
	}
	return result, nil, nil
}

// profileArgAsMap reads the user_profile argument as a map, accepting either a
// JSON string (how push rules send it) or a nested struct (manual invocation).
func profileArgAsMap(args *structpb.Struct, key string) (map[string]any, error) {
	if args == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	v, ok := args.GetFields()[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	switch k := v.GetKind().(type) {
	case *structpb.Value_StringValue:
		var m map[string]any
		if err := json.Unmarshal([]byte(k.StringValue), &m); err != nil {
			return nil, fmt.Errorf("invalid %s JSON: %w", key, err)
		}
		return m, nil
	case *structpb.Value_StructValue:
		return k.StructValue.AsMap(), nil
	default:
		return nil, fmt.Errorf("invalid %s format", key)
	}
}

// setEnabledHandler builds the handler for a lifecycle action that toggles a
// user's enabled flag (enabled=false for disable_user, true for enable_user).
// Idempotent; a 404 maps to NotFound and other errors keep their mapped gRPC code.
func (c *Connector) setEnabledHandler(actionName string, enabled bool) actions.ActionHandler {
	return func(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
		userID, err := userIDArg(args)
		if err != nil {
			return nil, nil, err
		}

		if err := c.client.SetUserEnabled(ctx, userID, enabled); err != nil {
			if client.IsNotFoundError(err) {
				return nil, nil, status.Errorf(codes.NotFound, "baton-keycloak: %s: user %s not found", actionName, userID)
			}
			return nil, nil, fmt.Errorf("baton-keycloak: %s user %s: %w", actionName, userID, err)
		}

		return successStruct(), nil, nil
	}
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

// successStruct returns the standard {"success": true} action response.
func successStruct() *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			retSuccessKey: {Kind: &structpb.Value_BoolValue{BoolValue: true}},
		},
	}
}
