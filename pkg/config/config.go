package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	keycloakServerURLField = field.StringField(
		"keycloak-server-url",
		field.WithDescription("The URL of the Keycloak server."),
		field.WithDefaultValue("https://keycloak.com/"),
		field.WithRequired(true),
	)
	keycloakRealmField = field.StringField(
		"keycloak-realm",
		field.WithDescription("The realm of the Keycloak server."),
		field.WithRequired(true),
	)
	keycloakClientIDField = field.StringField(
		"keycloak-client-id",
		field.WithDescription("The client ID you made."),
		field.WithRequired(true),
	)
	keycloakClientSecretField = field.StringField(
		"keycloak-client-secret",
		field.WithDescription("The client secret for the client you made."),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)
	syncSubGroupsField = field.BoolField(
		"sync-sub-groups",
		field.WithDescription("Enable syncing of sub-groups (nested groups). When enabled, the connector will sync the full group hierarchy."),
		field.WithDefaultValue(false),
	)
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	[]field.SchemaField{
		keycloakServerURLField,
		keycloakRealmField,
		keycloakClientIDField,
		keycloakClientSecretField,
		syncSubGroupsField,
	},
	field.WithConnectorDisplayName("Keycloak"),
	field.WithHelpUrl("/docs/baton/keycloak"),
	field.WithIconUrl("/static/app-icons/keycloak.svg"),
)
