# baton-keycloak
Welcome to your new connector! To start out, you will want to update the dependencies.
Do this by running `make update-deps`.

## Dev setup 

```
docker-compose up -d
# access http://localhost:8080 with admin login and admin password
```

# `baton-keycloak` Command Line Usage

```
baton-keycloak

Usage:
  baton-keycloak [flags]
  baton-keycloak [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  config             Get the connector config schema
  help               Help about any command

Flags:
      --client-id string                                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string                             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --external-resource-c1z string                     The path to the c1z file to sync external baton resources with ($BATON_EXTERNAL_RESOURCE_C1Z)
      --external-resource-entitlement-id-filter string   The entitlement that external users, groups must have access to sync external baton resources ($BATON_EXTERNAL_RESOURCE_ENTITLEMENT_ID_FILTER)
  -f, --file string                                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                                             help for baton-keycloak
      --keycloak-client-id string                        required: The client ID you made. ($BATON_KEYCLOAK_CLIENT_ID)
      --keycloak-client-secret string                    required: The client secret for the client you made. ($BATON_KEYCLOAK_CLIENT_SECRET)
      --keycloak-realm string                            required: The realm of the Keycloak server. ($BATON_KEYCLOAK_REALM)
      --keycloak-server-url string                       required: The URL of the Keycloak server. ($BATON_KEYCLOAK_SERVER_URL) (default "https://keycloak.com/")
      --log-format string                                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "console")
      --log-level string                                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --log-level-debug-expires-at string                The timestamp indicating when debug-level logging should expire ($BATON_LOG_LEVEL_DEBUG_EXPIRES_AT)
      --otel-collector-endpoint string                   The endpoint of the OpenTelemetry collector to send observability data to (used for both tracing and logging if specific endpoints are not provided) ($BATON_OTEL_COLLECTOR_ENDPOINT)
  -p, --provisioning                                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-entitlements-and-grants                     This must be set to skip syncing of entitlements and grants ($BATON_SKIP_ENTITLEMENTS_AND_GRANTS)
      --skip-full-sync                                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --sync-resources strings                           The resource IDs to sync ($BATON_SYNC_RESOURCES)
      --ticketing                                        This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                                          version for baton-keycloak

Use "baton-keycloak [command] --help" for more information about a command.
```