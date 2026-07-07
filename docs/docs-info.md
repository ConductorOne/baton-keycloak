# Keycloak Connector Setup Guide

---

## Requirements

- A **Keycloak** server (self-hosted or managed) with Admin REST API access
- A **confidential client** in the target realm with **Service accounts roles** enabled
- **realm-management** client roles on the service account: `manage-users`, `view-users`, `query-users`, `query-groups`

---

## Connector capabilities

1. **What resources does the connector sync?**
   This connector syncs:
   - Users (realm users from the Admin REST API)
   - Groups (top-level groups; nested groups when `--sync-sub-groups` is enabled)
   - Group membership (user → group grants, emitted from the group side)

2. **Can the connector provision any resources? If so, which ones?**
   The connector can provision:
   - User accounts via CreateAccount (POST /admin/realms/{realm}/users)
   - User deletion via Delete (DELETE /admin/realms/{realm}/users/{userId}) — permanent, hard delete
   - User enable/disable via the `enable_user` / `disable_user` actions (PUT /admin/realms/{realm}/users/{userId}, toggling `enabled`) — soft, reversible
   - User profile updates via the `update_user` action (PUT /admin/realms/{realm}/users/{userId}, `email` / `firstName` / `lastName`)
   - Group membership via Grant (PUT .../groups/{groupId}) and Revoke (DELETE .../groups/{groupId})

   It does **not** support realm/client role resources.

   **Disable vs. delete:** prefer `disable_user` to deactivate an account while preserving it (reversible via `enable_user`); use Delete only for permanent removal.

3. **Does the connector support grant expansion?**
   Yes, for nested groups when sub-group sync is enabled. Parent groups receive grants for members of child groups.

---

## Connector credentials

1. **What credentials or information are needed to set up the connector?**
   This connector requires:
   - Server URL of the Keycloak deployment
   - Realm to sync and provision
   - Confidential client ID and secret (service account)

   **Args**:
   - `--keycloak-server-url` — base URL of the Keycloak server (e.g. `https://auth.example.com`)
   - `--keycloak-realm` — realm to sync and provision
   - `--keycloak-client-id` — confidential client ID (service account)
   - `--keycloak-client-secret` — client secret
   - `--sync-sub-groups` — (optional) sync nested group hierarchies
   - `--provisioning` — (optional) enable Grant, Revoke, and CreateAccount

   The hidden `--base-url` flag overrides `--keycloak-server-url` for tests.

2. **For each item in the list above:**
   - **How does a user create or look up that credential or info?**

     **Server URL and realm:**
     1. Log in to the Keycloak Admin Console.
     2. The server URL is the HTTPS origin of the console; the realm is the realm you want to integrate (for example `master`).

     **Confidential client (service account):**
     1. In the Admin Console, go to **Clients** > **Create client** (OpenID Connect).
     2. Enable **Client authentication** to make the client confidential.
     3. On the client **Settings** tab, enable **Service accounts roles**.
     4. On the **Service account roles** tab, click **Assign role**, filter by **Client roles**, and select the **realm-management** client (named **master-realm** in the `master` realm).
     5. Assign `manage-users`, `view-users`, `query-users`, and `query-groups`.
     6. Copy the **Client secret** from the client **Credentials** tab.

   - **Does the credential need any specific scopes or permissions?**
     Keycloak uses **realm-management client roles**, not OAuth scopes. The service account needs `view-users` and `query-users` to sync users, `query-groups` to sync groups, and `manage-users` to create users and grant or revoke group membership.

   - **Is the list of scopes or permissions different to sync (read) versus provision (read-write)?**
     Yes. Sync requires the read roles (`view-users`, `query-users`, `query-groups`). Provisioning (CreateAccount, Grant, Revoke) additionally requires `manage-users`. A **403 Forbidden** on user creation means `manage-users` is missing.

   - **What level of access or permissions does the user need in order to create the credentials?**
     A Keycloak realm administrator who can create clients and assign realm-management roles to service accounts.

---

## Resource Details

### Users

- **Resource type ID**: `user`
- **Description**: Keycloak realm user
- **Traits**: User trait with login, email, and enabled status (profile: username, email, firstName, lastName)
- **Entitlements**: None (`SkipEntitlementsAndGrants` — membership grants are emitted from the group side)
- **Grants**: None (emitted from the group builder)
- **Provisioning**: CreateAccount (POST /admin/realms/{realm}/users). Credentials are set inline on create (atomic single call): **Random password** (preferred) writes a non-temporary password returned via `PlaintextData`; **No password** creates the user with a one-time `UPDATE_PASSWORD` required action. A duplicate username or email returns **409 Conflict**, which the connector treats as success (`AlreadyExistsResult`) after reading the user back by username, then by email. Existing accounts are not password-reset on idempotent retry. Delete (DELETE /admin/realms/{realm}/users/{userId}) permanently removes the user; a 404 (already gone) is treated as success.
- **Actions**:
  - `enable_user` / `disable_user` — toggle the user's `enabled` flag via PUT /admin/realms/{realm}/users/{userId} (the representation is read back first so only `enabled` changes). Both take a required `user_id` argument and are idempotent. Disable is a reversible soft deactivation; a disabled user surfaces as `STATUS_DISABLED` on the next sync.
  - `update_user` — updates a user's profile attributes (`email`, `firstName`, `lastName`) via PUT /admin/realms/{realm}/users/{userId}. Takes a required `user_id` plus a `user_profile` JSON object; only the keys present are changed (read-modify-write preserves the rest). A request with no updatable field is rejected as `InvalidArgument`. Registered as a global `ACTION_TYPE_ACCOUNT_UPDATE_PROFILE` action so C1 profile push rules can discover it.

### Groups

- **Resource type ID**: `group`
- **Description**: Keycloak realm group (top-level, plus nested groups when `--sync-sub-groups` is enabled)
- **Traits**: Group trait
- **Entitlements**: Static `member` entitlement shared by all groups, grantable to `user`
- **Grants**: One `member` grant per group member (principal: `user`)
- **Provisioning**: Grant (PUT .../users/{userId}/groups/{groupId}) and Revoke (DELETE .../users/{userId}/groups/{groupId})
- **Sub-groups**: When `--sync-sub-groups` is enabled, nested groups are listed under their parent. Keycloak 23+ uses `/groups/{id}/children`; older versions flatten the embedded `subGroups` from the parent list response.

---

## Authentication

- **OAuth2 client credentials** (service account) against the realm token endpoint.
- The connector exchanges the client ID and secret for an access token, then calls the Admin REST API with `Authorization: Bearer <token>`.
- Implemented with `github.com/Nerzal/gocloak/v13` and `github.com/Clarilab/gocloaksession` (token refresh handled by the session).

---

## API Endpoints Used

Keycloak Admin REST API (auth: Bearer token from client credentials):

- `POST   /realms/{realm}/protocol/openid-connect/token` — Obtain access token (client credentials)
- `GET    /admin/realms/{realm}/users?first=&max=` — List users
- `GET    /admin/realms/{realm}/users/{id}` — Fetch a single user (read-back after create)
- `POST   /admin/realms/{realm}/users` — Create user (account provisioning)
- `PUT    /admin/realms/{realm}/users/{id}` — Update user (enable/disable toggle `enabled`; `update_user` updates email/firstName/lastName)
- `DELETE /admin/realms/{realm}/users/{id}` — Delete user (hard delete / deprovision)
- `GET    /admin/realms/{realm}/users?username=&exact=true` — Look up user by username (409 read-back)
- `GET    /admin/realms/{realm}/users?email=&exact=true` — Look up user by email (409 read-back fallback)
- `GET    /admin/realms/{realm}/users/{id}/groups` — List a user's groups
- `PUT    /admin/realms/{realm}/users/{userId}/groups/{groupId}` — Add user to group (grant)
- `DELETE /admin/realms/{realm}/users/{userId}/groups/{groupId}` — Remove user from group (revoke)
- `GET    /admin/realms/{realm}/groups?first=&max=&subGroupsCount=true&briefRepresentation=false` — List groups
- `GET    /admin/realms/{realm}/groups/{id}/children?first=&max=` — List nested groups (Keycloak 23+)
- `GET    /admin/realms/{realm}/groups/{groupId}/members?first=&max=` — List group members (grants)
- `GET    /admin/serverinfo` — Detect the server major version (selects the sub-group strategy)

---

## Pagination

- Offset-based via `first` and `max` query params.
- Page size (`max`): `100`.
- The connector requests `max` items starting at `first`; when a page returns fewer than `max` items it is the last page, otherwise the next `first` is `first + len(page)`.

---

## Rate Limits

- Keycloak does not publish rate-limit headers. Request execution and retries follow the defaults of the underlying `gocloak` client; no connector-specific rate-limit handling is required.

---

## API Documentation

**Official Keycloak references:**

- **Admin REST API**: https://www.keycloak.org/docs-api/latest/rest-api/index.html
- **Service accounts (client credentials)**: https://www.keycloak.org/docs/latest/securing_apps/#_service_accounts
- **Server administration guide**: https://www.keycloak.org/docs/latest/server_admin/

**Local development:**

- A `docker-compose.yml` at the repo root runs Keycloak plus Postgres. `--keycloak-server-url` must point at your deployment — there is no fixed port or host; use whatever your instance exposes (for the local compose stack, `docker compose port keycloak 8080`).
