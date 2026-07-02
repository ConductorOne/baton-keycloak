# `baton-keycloak` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-keycloak.svg)](https://pkg.go.dev/github.com/conductorone/baton-keycloak) ![ci](https://github.com/conductorone/baton-keycloak/actions/workflows/ci.yaml/badge.svg)

`baton-keycloak` is a connector for [Keycloak](https://www.keycloak.org) built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Keycloak Admin REST API to sync users and groups and to provision accounts and group membership.

Check out [Baton](https://github.com/conductorone/baton) to learn more about the project in general.

# Getting Started

## Prerequisites

- A [Keycloak](https://www.keycloak.org) realm with Admin REST API access
- A Keycloak realm administrator who can create a [confidential client with service account roles](https://www.keycloak.org/docs/latest/server_admin/#_service_accounts)
- **realm-management** client roles: `manage-users`, `view-users`, `query-users`, `query-groups`

See the internal [Setup Guide](./docs/docs-info.md) for credential steps and API details.

## Local Keycloak (docker-compose)

```bash
docker compose up -d
```

Create a confidential client with service account roles before running the connector.

## brew

```bash
brew install conductorone/baton/baton conductorone/baton/baton-keycloak

baton-keycloak \
  --keycloak-server-url="$KEYCLOAK_SERVER_URL" \
  --keycloak-realm="$KEYCLOAK_REALM" \
  --keycloak-client-id="$KEYCLOAK_CLIENT_ID" \
  --keycloak-client-secret="$KEYCLOAK_CLIENT_SECRET" \
  --sync-sub-groups \
  --file=sync.c1z

baton resources --file=sync.c1z
baton entitlements --file=sync.c1z
baton grants --file=sync.c1z
```

## docker

```bash
docker run --rm -v $(pwd):/out \
  -e BATON_KEYCLOAK_SERVER_URL="$KEYCLOAK_SERVER_URL" \
  -e BATON_KEYCLOAK_REALM="$KEYCLOAK_REALM" \
  -e BATON_KEYCLOAK_CLIENT_ID="$KEYCLOAK_CLIENT_ID" \
  -e BATON_KEYCLOAK_CLIENT_SECRET="$KEYCLOAK_CLIENT_SECRET" \
  -e BATON_SYNC_SUB_GROUPS=true \
  ghcr.io/conductorone/baton-keycloak:latest -f "/out/sync.c1z"

docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```bash
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-keycloak/cmd/baton-keycloak@main

baton-keycloak \
  --keycloak-server-url="$KEYCLOAK_SERVER_URL" \
  --keycloak-realm="$KEYCLOAK_REALM" \
  --keycloak-client-id="$KEYCLOAK_CLIENT_ID" \
  --keycloak-client-secret="$KEYCLOAK_CLIENT_SECRET" \
  --sync-sub-groups

baton resources
```

# Data Model

`baton-keycloak` syncs the following resources:

- **Users** — `GET /admin/realms/{realm}/users`
- **Groups** — `GET /admin/realms/{realm}/groups` with a static `member` entitlement
- **Nested groups** — optional via `--sync-sub-groups` (sub-children API on Keycloak 23+)

For endpoints, pagination, and implementation rationale, see the [Setup Guide](./docs/docs-info.md).

# Provisioning

Enable provisioning with the `--provisioning` flag (or `BATON_PROVISIONING=true` in service mode).

## Account Management

- **Create account** — `POST /admin/realms/{realm}/users` when C1 needs a new AppUser (`CreateAccount`). Supports random password (preferred) and no password (`UPDATE_PASSWORD` required action).
- **Delete account** — `DELETE /admin/realms/{realm}/users/{userId}` (`Delete`). Permanent hard delete; a 404 (already gone) is treated as success.

## Actions

- **`enable_user`** — reactivates a user (`PUT /admin/realms/{realm}/users/{userId}` with `enabled=true`).
- **`disable_user`** — deactivates a user, reversibly (`PUT /admin/realms/{realm}/users/{userId}` with `enabled=false`).

Both take a required `user_id` argument and are idempotent. Invoke with `--invoke-action=<name> --invoke-action-args='{"user_id":"<id>"}'` (no `-p` flag needed).

## Entitlement Management

- **Grant group membership** — `PUT /admin/realms/{realm}/users/{userId}/groups/{groupId}`
- **Revoke group membership** — `DELETE /admin/realms/{realm}/users/{userId}/groups/{groupId}`

Customer-facing setup: [`docs/connector.mdx`](./docs/connector.mdx).

# Contributing, Support and Issues

We welcome contributions and ideas. If you have questions, problems, or ideas, open a GitHub Issue.

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

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
  health-check       Check the health of a running connector
  help               Help about any command

Flags:
      --auth-method string                               ($BATON_AUTH_METHOD)
      --client-id string                                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string                             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --external-resource-c1z string                     The path to the c1z file to sync external baton resources with ($BATON_EXTERNAL_RESOURCE_C1Z)
      --external-resource-entitlement-id-filter string   The entitlement that external users, groups must have access to sync external baton resources ($BATON_EXTERNAL_RESOURCE_ENTITLEMENT_ID_FILTER)
  -f, --file string                                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --health-check                                     Enable the HTTP health check endpoint ($BATON_HEALTH_CHECK)
      --health-check-port int                            Port for the HTTP health check endpoint ($BATON_HEALTH_CHECK_PORT) (default 8081)
  -h, --help                                             help for baton-keycloak
      --http-timeout-seconds int                         HTTP client timeout in seconds (max 1800) ($BATON_HTTP_TIMEOUT_SECONDS) (default 300)
      --keep-previous-sync-c1z                           Keep the previously synced c1z on disk to enable ETag replay across service-mode syncs (requires a connector that supports ETag replay; costs one c1z of local disk) ($BATON_KEEP_PREVIOUS_SYNC_C1Z)
      --keycloak-client-id string                        required: The client ID you made. ($BATON_KEYCLOAK_CLIENT_ID)
      --keycloak-client-secret string                    required: The client secret for the client you made. ($BATON_KEYCLOAK_CLIENT_SECRET)
      --keycloak-realm string                            required: The realm of the Keycloak server. ($BATON_KEYCLOAK_REALM)
      --keycloak-server-url string                       required: The URL of the Keycloak server. ($BATON_KEYCLOAK_SERVER_URL) (default "https://keycloak.com/")
      --log-format string                                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --log-level-debug-expires-at string                The timestamp indicating when debug-level logging should expire ($BATON_LOG_LEVEL_DEBUG_EXPIRES_AT)
      --otel-collector-endpoint string                   The endpoint of the OpenTelemetry collector to send observability data to (used for both tracing and logging if specific endpoints are not provided) ($BATON_OTEL_COLLECTOR_ENDPOINT)
      --parallel-sync                                    Deprecated: use --workers instead. ($BATON_PARALLEL_SYNC)
  -p, --provisioning                                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-entitlements-and-grants                     This must be set to skip syncing of entitlements and grants ($BATON_SKIP_ENTITLEMENTS_AND_GRANTS)
      --skip-full-sync                                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --storage-engine string                            The storage engine to use when opening the sync c1z file: sqlite or pebble. Leave unset to use the baton-sdk default. ($BATON_STORAGE_ENGINE)
      --sync-resource-types strings                      The resource type IDs to sync ($BATON_SYNC_RESOURCE_TYPES)
      --sync-resources strings                           The resource IDs to sync ($BATON_SYNC_RESOURCES)
      --sync-sub-groups                                  Enable syncing of sub-groups (nested groups). When enabled, the connector will sync the full group hierarchy. ($BATON_SYNC_SUB_GROUPS)
      --task-concurrency int                             The number of Baton tasks to run concurrently in service mode. Tasks may include sync, grant, revoke, and more. Minimum value is 1, maximum value is 100. ($BATON_TASK_CONCURRENCY) (default 3)
      --ticketing                                        This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                                          version for baton-keycloak
      --workers int                                      The number of sync workers to use. -1 for auto-detect, 0 for sequential, >0 for parallel ($BATON_WORKERS)

Use "baton-keycloak [command] --help" for more information about a command.
```