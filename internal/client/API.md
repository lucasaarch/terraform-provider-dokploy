# Dokploy API Reference

> Source of truth for the Terraform provider client implementation.  
> Verified against live instance on 2026-05-22.  
> Base URL: `<DOKPLOY_ENDPOINT>/api`  
> Auth: all requests require header `x-api-key: <token>`  
> Protocol: reads are GET with query params; mutations are POST with JSON body (`Content-Type: application/json`).

---

## OpenAPI Spec

Not served. The following paths return 404: `/swagger.json`, `/api/swagger.json`, `/api/openapi.json`, `/openapi.json`, `/api/swagger/json`, `/api/docs/json`. The Swagger UI HTML page is available at `GET /swagger` but no machine-readable spec was found.

---

## organization.*

### `GET /api/organization.all`

Returns all organizations the authenticated API key has access to.

**Request:** no body, no query params.

**Response:** `200 application/json` — array of organization objects.

```json
[
  {
    "id": "BTFAI_7TzbiGeXtbPMTT-",
    "name": "Blitz IT Solutions",
    "slug": null,
    "logo": "",
    "createdAt": "2026-04-08T04:29:05.237Z",
    "metadata": null,
    "ownerId": "Sw9oCKSqGUHWs8OdOrdxvTWrQIS8HZsF",
    "members": [
      {
        "id": "4vT5stBrJYujZ3NejlCvq",
        "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
        "userId": "Sw9oCKSqGUHWs8OdOrdxvTWrQIS8HZsF",
        "role": "owner",
        "createdAt": "...",
        "teamId": null,
        "isDefault": true,
        "canCreateProjects": false,
        "canAccessToSSHKeys": false,
        "canCreateServices": false,
        "canDeleteProjects": false,
        "canDeleteServices": false,
        "canAccessToDocker": false,
        "canAccessToAPI": false,
        "canAccessToGitProviders": false,
        "canAccessToTraefikFiles": false,
        "canDeleteEnvironments": false,
        "canCreateEnvironments": false,
        "accessedProjects": ["..."],
        "accessedEnvironments": ["..."],
        "accessedServices": ["..."],
        "accessedGitProviders": [],
        "accessedServers": []
      }
    ]
  }
]
```

**Key fields on each organization:**
| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Organization ID (used in project `organizationId`) |
| `name` | string | Display name |
| `slug` | string\|null | URL slug (may be null) |
| `logo` | string | URL or empty string |
| `ownerId` | string | User ID of owner |
| `members` | array | Member records with permissions |

**Notes:**
- Organizations cannot be created or modified via the API — they are managed through the Dokploy UI.
- The API key may have access to multiple organizations (verified: 2 organizations returned for this instance).
- The primary key is `id` (not `organizationId`) — this field is used as `organizationId` in project objects.

---

## project.*

### `GET /api/project.all`

Returns all projects across all organizations the key has access to.

**Request:** no body, no query params.

**Response:** `200 application/json` — array of project objects (with embedded environments).

```json
[
  {
    "projectId": "hMmo6riozqlgz6NRsbOTy",
    "name": "my-project",
    "description": "A project",
    "createdAt": "2026-05-22T21:42:46.542Z",
    "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
    "env": "",
    "environments": [
      {
        "name": "production",
        "environmentId": "tyth5yNvYyZ2CBu0WG9ZX",
        "isDefault": true,
        "applications": [
          {
            "applicationId": "9vP7oCl1GSwYgNBHt5cvj",
            "name": "my-app",
            "applicationStatus": "idle"
          }
        ]
      }
    ],
    "projectTags": []
  }
]
```

Note: `project.all` returns a summary of each environment's applications (id, name, applicationStatus only). Use `application.one` for the full application object.

---

### `GET /api/project.one?projectId=<id>`

Returns a single project with full environment details.

**Query params:** `projectId` (string, required).

**Response:** `200 application/json` — single project object.

```json
{
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "name": "my-project",
  "description": "temporary probe",
  "createdAt": "2026-05-22T21:47:31.437Z",
  "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
  "env": "",
  "environments": [
    {
      "environmentId": "LaH8mtSvi1JiFl7jrN5E7",
      "name": "production",
      "description": "Production environment",
      "createdAt": "...",
      "env": "",
      "projectId": "uxT7lhTkOGfvIQaOnTH2I",
      "isDefault": true,
      "applications": [],
      "compose": [],
      "libsql": [],
      "mariadb": [],
      "mongo": [],
      "mysql": [],
      "postgres": [],
      "redis": []
    }
  ],
  "projectTags": []
}
```

---

### `POST /api/project.create`

Creates a new project. **Auto-creates a default `production` environment.**

**Request body:**
```json
{
  "name": "my-project",
  "description": "optional description"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Project display name |
| `description` | string | no | Project description |

**Response:** `200 application/json`

```json
{
  "project": {
    "projectId": "uxT7lhTkOGfvIQaOnTH2I",
    "name": "my-project",
    "description": "optional description",
    "createdAt": "2026-05-22T21:47:31.437Z",
    "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
    "env": ""
  },
  "environment": {
    "environmentId": "LaH8mtSvi1JiFl7jrN5E7",
    "name": "production",
    "description": "Production environment",
    "createdAt": "2026-05-22T21:47:31.454Z",
    "env": "",
    "projectId": "uxT7lhTkOGfvIQaOnTH2I",
    "isDefault": true
  }
}
```

**Important:** The response has two top-level keys: `project` and `environment`. The auto-created default environment is returned inline.

---

### `POST /api/project.update`

Updates a project's name, description, and/or environment variables.

**Request body:**
```json
{
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "name": "new-name",
  "description": "new description",
  "env": "FOO=bar\nBAZ=qux"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `projectId` | string | yes | ID of the project to update |
| `name` | string | no | New display name |
| `description` | string | no | New description |
| `env` | string | no | Project-level env vars (newline-separated KEY=value) |

**Response:** `200 application/json` — updated project object (without environments):

```json
{
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "name": "new-name",
  "description": "new description",
  "createdAt": "2026-05-22T21:47:31.437Z",
  "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
  "env": "FOO=bar\nBAZ=qux"
}
```

---

### `POST /api/project.remove`

Deletes a project and all its environments and services.

**Request body:**
```json
{
  "projectId": "uxT7lhTkOGfvIQaOnTH2I"
}
```

**Response:** `200 application/json` — the deleted project object (without environments).

---

## environment.*

**Risk item confirmed:** The `environment.*` router exists and is fully functional.

### `GET /api/environment.one?environmentId=<id>`

Returns a single environment with its associated services and parent project.

**Query params:** `environmentId` (string, required).

**Response:** `200 application/json`

```json
{
  "name": "production",
  "description": "Production environment",
  "environmentId": "LaH8mtSvi1JiFl7jrN5E7",
  "isDefault": true,
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "env": "",
  "applications": [],
  "mariadb": [],
  "mongo": [],
  "mysql": [],
  "postgres": [],
  "redis": [],
  "compose": [],
  "libsql": [],
  "project": {
    "projectId": "uxT7lhTkOGfvIQaOnTH2I",
    "name": "my-project",
    "description": "updated description",
    "createdAt": "2026-05-22T21:47:31.437Z",
    "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
    "env": "FOO=bar"
  }
}
```

---

### `POST /api/environment.create`

Creates a new (non-default) environment within a project.

**Request body:**
```json
{
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "name": "staging",
  "description": "optional description"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `projectId` | string | yes | Parent project ID |
| `name` | string | yes | Environment name |
| `description` | string | no | Environment description |

**Response:** `200 application/json`

```json
{
  "environmentId": "-4fBQOXJ4xSZVrWy19n2e",
  "name": "staging",
  "description": "optional description",
  "createdAt": "2026-05-22T21:47:47.190Z",
  "env": "",
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "isDefault": false
}
```

---

### `POST /api/environment.update`

Updates an environment's name, description, and/or environment variables.

**Request body:**
```json
{
  "environmentId": "-4fBQOXJ4xSZVrWy19n2e",
  "name": "staging-v2",
  "description": "updated description",
  "env": "BAR=baz"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `environmentId` | string | yes | ID of environment to update |
| `name` | string | no | New name |
| `description` | string | no | New description |
| `env` | string | no | Env vars (newline-separated KEY=value) |

**Response:** `200 application/json` — updated environment object:

```json
{
  "environmentId": "-4fBQOXJ4xSZVrWy19n2e",
  "name": "staging-v2",
  "description": "updated description",
  "createdAt": "...",
  "env": "BAR=baz",
  "projectId": "uxT7lhTkOGfvIQaOnTH2I",
  "isDefault": false
}
```

---

### `POST /api/environment.remove`

Deletes an environment and all its services.

**Request body:**
```json
{
  "environmentId": "-4fBQOXJ4xSZVrWy19n2e"
}
```

**Response:** `200 application/json` — the deleted environment object.

---

## application.*

### `GET /api/application.one?applicationId=<id>`

Returns a single application with full details including domains, deployments, and relations.

**Query params:** `applicationId` (string, required).

**Response:** `200 application/json` — full application object. Key fields:

```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "name": "my-app",
  "appName": "my-app-yg3mir",
  "description": "probe application",
  "env": "APP_ENV=production",
  "applicationStatus": "done",
  "sourceType": "docker",
  "dockerImage": "nginx:alpine",
  "registryUrl": "",
  "username": null,
  "password": null,
  "registryId": null,
  "buildType": "nixpacks",
  "replicas": 1,
  "autoDeploy": true,
  "triggerType": "push",
  "environmentId": "LaH8mtSvi1JiFl7jrN5E7",
  "createdAt": "2026-05-22T21:48:00.081Z",
  "refreshToken": "vpF2geRZ-4vEZatmCLluc",
  "buildArgs": null,
  "buildSecrets": null,
  "createEnvFile": true,
  "dockerfile": "Dockerfile",
  "buildPath": "/",
  "domains": [],
  "deployments": [
    {
      "deploymentId": "...",
      "status": "done",
      "title": "Manual deployment",
      "createdAt": "...",
      "startedAt": "...",
      "finishedAt": "..."
    }
  ],
  "environment": {
    "environmentId": "...",
    "name": "production",
    "projectId": "...",
    "isDefault": true,
    "project": { "..." : "..." }
  }
}
```

**Complete field list (all present on every response):**
`applicationId`, `name`, `appName`, `description`, `env`, `previewEnv`, `watchPaths`, `previewBuildArgs`, `previewBuildSecrets`, `previewLabels`, `previewWildcard`, `previewPort`, `previewHttps`, `previewPath`, `previewCertificateType`, `previewCustomCertResolver`, `previewLimit`, `isPreviewDeploymentsActive`, `previewRequireCollaboratorPermissions`, `rollbackActive`, `buildArgs`, `buildSecrets`, `memoryReservation`, `memoryLimit`, `cpuReservation`, `cpuLimit`, `title`, `enabled`, `subtitle`, `command`, `args`, `icon`, `refreshToken`, `sourceType`, `cleanCache`, `repository`, `owner`, `branch`, `buildPath`, `triggerType`, `autoDeploy`, `gitlabProjectId`, `gitlabRepository`, `gitlabOwner`, `gitlabBranch`, `gitlabBuildPath`, `gitlabPathNamespace`, `giteaRepository`, `giteaOwner`, `giteaBranch`, `giteaBuildPath`, `bitbucketRepository`, `bitbucketRepositorySlug`, `bitbucketOwner`, `bitbucketBranch`, `bitbucketBuildPath`, `username`, `password`, `dockerImage`, `registryUrl`, `customGitUrl`, `customGitBranch`, `customGitBuildPath`, `customGitSSHKeyId`, `enableSubmodules`, `dockerfile`, `dockerContextPath`, `dockerBuildStage`, `dropBuildPath`, `healthCheckSwarm`, `restartPolicySwarm`, `placementSwarm`, `updateConfigSwarm`, `rollbackConfigSwarm`, `modeSwarm`, `labelsSwarm`, `networkSwarm`, `stopGracePeriodSwarm`, `endpointSpecSwarm`, `ulimitsSwarm`, `replicas`, `applicationStatus`, `buildType`, `railpackVersion`, `herokuVersion`, `publishDirectory`, `isStaticSpa`, `createEnvFile`, `createdAt`, `registryId`, `rollbackRegistryId`, `environmentId`, `githubId`, `gitlabId`, `giteaId`, `bitbucketId`, `serverId`, `buildServerId`, `buildRegistryId`, `environment`, `domains`, `deployments`, `mounts`, `redirects`, `security`, `ports`, `registry`, `gitlab`, `github`, `bitbucket`, `gitea`, `server`, `previewDeployments`, `buildRegistry`, `rollbackRegistry`, `hasGitProviderAccess`, `unauthorizedProvider`

**Risk item confirmed:** `application.one` does NOT return `registryPassword`. Docker registry credentials are stored as `username` (string|null) and `password` (string|null). These fields ARE returned by `application.one` but are `null` when no private registry is configured. There is no separate `registryPassword` field.

---

### `POST /api/application.create`

Creates a new application in an environment.

**Request body:**
```json
{
  "name": "my-app",
  "appName": "my-app",
  "environmentId": "LaH8mtSvi1JiFl7jrN5E7",
  "description": "optional description"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (will be suffixed with random chars, e.g. `my-app-yg3mir`) |
| `environmentId` | string | yes | Parent environment ID |
| `description` | string | no | Description |

**Response:** `200 application/json` — the full application object (same shape as `application.one` minus the relation sub-objects).

**Note:** `appName` in the response is the auto-generated unique name (e.g. `my-app-yg3mir`), not the input value verbatim.

---

### `POST /api/application.update`

Updates application metadata fields.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "name": "new-name",
  "description": "new description",
  "dockerImage": "nginx:alpine",
  "sourceType": "docker"
}
```

Any writable field from the application object may be included. `applicationId` is always required.

**Response:** `200 application/json` — `true` (boolean, not the updated object).

---

### `POST /api/application.saveDockerProvider`

Sets the Docker image source configuration for an application.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "dockerImage": "nginx:alpine",
  "username": null,
  "password": null,
  "registryUrl": ""
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `applicationId` | string | yes | Application ID |
| `dockerImage` | string | yes | Image reference (e.g. `nginx:alpine`) |
| `username` | string\|null | yes | Registry username (null for public registries) |
| `password` | string\|null | yes | Registry password (null for public registries) |
| `registryUrl` | string | **yes** | Registry URL — must be present even if empty string |

**Response:** `200 application/json` — `true`.

**Note:** `registryUrl` is required by the server's Zod validation even for public Docker Hub images — pass `""` (empty string) for public images.

---

### `POST /api/application.saveEnvironment`

Sets the application's runtime environment variables and build-time settings.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "env": "APP_ENV=production\nDEBUG=false",
  "buildArgs": null,
  "buildSecrets": null,
  "createEnvFile": true
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `applicationId` | string | yes | Application ID |
| `env` | string | yes | Newline-separated KEY=value pairs |
| `buildArgs` | string\|null | **yes** | Build arguments (null if none) — required by Zod |
| `buildSecrets` | string\|null | **yes** | Build secrets (null if none) — required by Zod |
| `createEnvFile` | boolean | **yes** | Whether to write a `.env` file — required by Zod |

**Response:** `200 application/json` — `true`.

**Note:** `buildArgs`, `buildSecrets`, and `createEnvFile` are all required by server Zod validation even if not relevant.

---

### `POST /api/application.deploy`

Triggers a deployment for an application.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK"
}
```

**Response:** `200 application/json` — empty body (no content returned).

---

### `POST /api/application.delete`

Deletes an application and all its associated resources.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK"
}
```

**Response:** `200 application/json` — the full application object (including relations) at the time of deletion.

---

## domain.*

### `GET /api/domain.one?domainId=<id>`

Returns a single domain with its associated application.

**Query params:** `domainId` (string, required).

**Response:** `200 application/json`

```json
{
  "domainId": "PHFKu7Zyax7JvK-ShG_IE",
  "host": "myapp.example.com",
  "https": false,
  "port": 80,
  "path": "/",
  "certificateType": "none",
  "internalPath": "/",
  "stripPath": false,
  "domainType": "application",
  "customEntrypoint": null,
  "customCertResolver": null,
  "serviceName": null,
  "uniqueConfigKey": 45,
  "createdAt": "2026-05-22T21:48:53.258Z",
  "composeId": null,
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "previewDeploymentId": null,
  "middlewares": [],
  "application": { "...": "..." }
}
```

---

### `POST /api/domain.create`

Creates a domain/routing rule for an application.

**Request body:**
```json
{
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "host": "myapp.example.com",
  "port": 80,
  "https": false,
  "path": "/",
  "certificateType": "none"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `applicationId` | string | yes | Target application ID |
| `host` | string | yes | Hostname (e.g. `myapp.example.com`) |
  | `port` | integer | yes | Target container port |
| `https` | boolean | yes | Whether to enable HTTPS redirect |
| `path` | string | yes | URL path prefix (use `"/"` for root) |
| `certificateType` | string | yes | `"none"`, `"letsencrypt"`, or `"custom"` |

**Response:** `200 application/json` — the created domain object:

```json
{
  "domainId": "PHFKu7Zyax7JvK-ShG_IE",
  "host": "myapp.example.com",
  "https": false,
  "port": 80,
  "customEntrypoint": null,
  "path": "/",
  "serviceName": null,
  "domainType": "application",
  "uniqueConfigKey": 45,
  "createdAt": "2026-05-22T21:48:53.258Z",
  "composeId": null,
  "customCertResolver": null,
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK",
  "previewDeploymentId": null,
  "certificateType": "none",
  "internalPath": "/",
  "stripPath": false,
  "middlewares": []
}
```

---

### `POST /api/domain.update`

Updates a domain's configuration.

**Request body:**
```json
{
  "domainId": "PHFKu7Zyax7JvK-ShG_IE",
  "host": "new-host.example.com",
  "port": 8080,
  "https": false,
  "path": "/",
  "certificateType": "none"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `domainId` | string | yes | Domain ID to update |
| `host` | string | no | New hostname |
| `port` | integer | no | New port |
| `https` | boolean | no | HTTPS toggle |
| `path` | string | no | URL path prefix |
| `certificateType` | string | no | Certificate type |

**Response:** `200 application/json` — updated domain object (same shape as `domain.create` response, without `application` sub-object).

---

### `POST /api/domain.delete`

Deletes a domain.

**Request body:**
```json
{
  "domainId": "PHFKu7Zyax7JvK-ShG_IE"
}
```

**Response:** `200 application/json` — the deleted domain object.

---

## postgres.*

> Verified against live instance on 2026-05-22.

**Note on `postgres.all`:** This endpoint does NOT exist (returns 404). Postgres instances are listed via `project.one` or `environment.one` (the `postgres` array on each environment object).

### `GET /api/postgres.one?postgresId=<id>`

Returns a single postgres instance with full details including mounts, environment, and backups.

**Query params:** `postgresId` (string, required).

**Response:** `200 application/json`

```json
{
  "postgresId": "nrxOqifKRRjXfU2P42Whu",
  "name": "my-postgres",
  "appName": "my-postgres-hlbgau",
  "description": null,
  "databaseName": "probedb",
  "databaseUser": "probeuser",
  "databasePassword": "probepass1234",
  "dockerImage": "postgres:18",
  "command": null,
  "args": null,
  "env": null,
  "externalPort": null,
  "memoryReservation": null,
  "memoryLimit": null,
  "cpuReservation": null,
  "cpuLimit": null,
  "replicas": 1,
  "applicationStatus": "done",
  "createdAt": "2026-05-23T01:32:44.116Z",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "serverId": null,
  "healthCheckSwarm": null,
  "restartPolicySwarm": null,
  "placementSwarm": null,
  "updateConfigSwarm": null,
  "rollbackConfigSwarm": null,
  "modeSwarm": null,
  "labelsSwarm": null,
  "networkSwarm": null,
  "stopGracePeriodSwarm": null,
  "endpointSpecSwarm": null,
  "ulimitsSwarm": null,
  "environment": {
    "environmentId": "...",
    "name": "production",
    "projectId": "...",
    "isDefault": true,
    "project": { "...": "..." }
  },
  "mounts": [
    {
      "mountId": "uRfv9BwzL52wxjhFxnJKH",
      "type": "volume",
      "volumeName": "my-postgres-hlbgau-data",
      "mountPath": "/var/lib/postgresql/18/docker",
      "serviceType": "postgres",
      "postgresId": "nrxOqifKRRjXfU2P42Whu"
    }
  ],
  "server": null,
  "backups": []
}
```

**Password in read:** `databasePassword` IS returned in plaintext by `postgres.one`. The Terraform provider must treat this field as sensitive and use the state value for drift detection.

---

### `POST /api/postgres.create`

Creates a new postgres database service.

**Request body:**
```json
{
  "name": "my-postgres",
  "appName": "my-postgres",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "databaseName": "mydb",
  "databaseUser": "myuser",
  "databasePassword": "mypassword"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (auto-suffixed with random chars, e.g. `my-postgres-hlbgau`) |
| `environmentId` | string | yes | Parent environment ID |
| `databaseName` | string | yes | Name of the default database to create |
| `databaseUser` | string | yes | Database superuser name |
| `databasePassword` | string | yes | Database superuser password |
| `description` | string | no | Optional description |
| `dockerImage` | string | no | Defaults to `postgres:18` if omitted |

**Response:** `200 application/json` — the created postgres object (same shape as `postgres.one` minus the `environment`, `mounts`, `server`, `backups` sub-objects).

---

### `POST /api/postgres.update`

Updates a postgres instance's configuration.

**Request body:**
```json
{
  "postgresId": "nrxOqifKRRjXfU2P42Whu",
  "description": "updated description",
  "externalPort": 5432,
  "dockerImage": "postgres:16",
  "env": "PGDATA=/data",
  "command": null
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `postgresId` | string | yes | ID of the postgres instance to update |
| any other field | varies | no | Any writable field from the postgres object |

**Response:** `200 application/json` — `true` (boolean).

---

### `POST /api/postgres.deploy`

Triggers a deployment (container start/restart) for a postgres instance.

**Request body:**
```json
{
  "postgresId": "nrxOqifKRRjXfU2P42Whu"
}
```

**Response:** `200 application/json` — the **full postgres object** (same shape as `postgres.one`) with `applicationStatus` reflecting the state at the moment the deploy was enqueued (typically `idle` or `running`). Poll `postgres.one` until `applicationStatus` is `done` or `error`.

**Key difference from `application.deploy`:** Database deploy returns the full object; application deploy returns an empty body.

---

### `POST /api/postgres.remove`

Deletes a postgres instance and its associated volumes.

**Request body:**
```json
{
  "postgresId": "nrxOqifKRRjXfU2P42Whu"
}
```

**Response:** `200 application/json` — the deleted postgres object.

---

### Observed `applicationStatus` transitions (postgres)

| Transition | Status |
|-----------|--------|
| After `postgres.create` | `idle` |
| Immediately after `postgres.deploy` | `idle` (deploy is async; status updates server-side) |
| After deploy completes (postgres:18, ~3–5s) | `done` |
| After deploy fails | `error` |

---

## mysql.*

> Verified against live instance on 2026-05-22.

**Note on `mysql.all`:** Does NOT exist (returns 404). MySQL instances are listed via `project.one` or `environment.one` (the `mysql` array).

### `GET /api/mysql.one?mysqlId=<id>`

Returns a single mysql instance.

**Query params:** `mysqlId` (string, required).

**Response:** `200 application/json`

```json
{
  "mysqlId": "6ExRn6o2bsiX2c_7c2CZQ",
  "name": "my-mysql",
  "appName": "my-mysql-iykhey",
  "description": null,
  "databaseName": "mydb",
  "databaseUser": "myuser",
  "databasePassword": "mypassword",
  "databaseRootPassword": "fhkymm6ximwldgwj",
  "dockerImage": "mysql:8",
  "command": null,
  "args": null,
  "env": null,
  "externalPort": null,
  "memoryReservation": null,
  "memoryLimit": null,
  "cpuReservation": null,
  "cpuLimit": null,
  "replicas": 1,
  "applicationStatus": "done",
  "createdAt": "2026-05-23T01:32:52.618Z",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "serverId": null,
  "healthCheckSwarm": null,
  "restartPolicySwarm": null,
  "placementSwarm": null,
  "updateConfigSwarm": null,
  "rollbackConfigSwarm": null,
  "modeSwarm": null,
  "labelsSwarm": null,
  "networkSwarm": null,
  "stopGracePeriodSwarm": null,
  "endpointSpecSwarm": null,
  "ulimitsSwarm": null,
  "environment": { "...": "..." },
  "mounts": [
    {
      "mountId": "1wFwkUSVqRoecOVcN21P4",
      "type": "volume",
      "volumeName": "my-mysql-iykhey-data",
      "mountPath": "/var/lib/mysql",
      "serviceType": "mysql",
      "mysqlId": "6ExRn6o2bsiX2c_7c2CZQ"
    }
  ],
  "server": null,
  "backups": []
}
```

**Password in read:** Both `databasePassword` and `databaseRootPassword` ARE returned in plaintext by `mysql.one`. Treat both as sensitive.

**`databaseRootPassword` note:** If not supplied on create, Dokploy auto-generates a random root password. The generated value is returned in the create response and by `mysql.one`.

---

### `POST /api/mysql.create`

**Request body:**
```json
{
  "name": "my-mysql",
  "appName": "my-mysql",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "databaseName": "mydb",
  "databaseUser": "myuser",
  "databasePassword": "mypassword"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (auto-suffixed) |
| `environmentId` | string | yes | Parent environment ID |
| `databaseName` | string | yes | Database name |
| `databaseUser` | string | yes | MySQL user name |
| `databasePassword` | string | yes | MySQL user password |
| `databaseRootPassword` | string | no | Root password — auto-generated if omitted |
| `description` | string | no | Optional description |
| `dockerImage` | string | no | Defaults to `mysql:8` |

**Response:** `200 application/json` — the created mysql object (includes auto-generated `databaseRootPassword`).

---

### `POST /api/mysql.update`

**Request body:**
```json
{
  "mysqlId": "6ExRn6o2bsiX2c_7c2CZQ",
  "description": "updated",
  "externalPort": 3306
}
```

**Response:** `200 application/json` — `true`.

---

### `POST /api/mysql.deploy`

**Request body:** `{ "mysqlId": "6ExRn6o2bsiX2c_7c2CZQ" }`

**Response:** `200 application/json` — full mysql object (same as `mysql.one`). Poll until `applicationStatus` is `done` or `error`.

---

### `POST /api/mysql.remove`

**Request body:** `{ "mysqlId": "6ExRn6o2bsiX2c_7c2CZQ" }`

**Response:** `200 application/json` — the deleted mysql object.

---

## mariadb.*

> Verified against live instance on 2026-05-22.

**Note on `mariadb.all`:** Does NOT exist (returns 404). MariaDB instances are listed via `project.one` or `environment.one` (the `mariadb` array).

### `GET /api/mariadb.one?mariadbId=<id>`

**Query params:** `mariadbId` (string, required).

**Response:** `200 application/json`

```json
{
  "mariadbId": "SfJMoi2CiK0_03YiYg6hx",
  "name": "my-mariadb",
  "appName": "my-mariadb-ot1nlc",
  "description": null,
  "databaseName": "mydb",
  "databaseUser": "myuser",
  "databasePassword": "mypassword",
  "databaseRootPassword": "hj88fnieewxo0acx",
  "dockerImage": "mariadb:11",
  "command": null,
  "args": null,
  "env": null,
  "externalPort": null,
  "memoryReservation": null,
  "memoryLimit": null,
  "cpuReservation": null,
  "cpuLimit": null,
  "replicas": 1,
  "applicationStatus": "done",
  "createdAt": "2026-05-23T01:32:59.155Z",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "serverId": null,
  "healthCheckSwarm": null,
  "restartPolicySwarm": null,
  "placementSwarm": null,
  "updateConfigSwarm": null,
  "rollbackConfigSwarm": null,
  "modeSwarm": null,
  "labelsSwarm": null,
  "networkSwarm": null,
  "stopGracePeriodSwarm": null,
  "endpointSpecSwarm": null,
  "ulimitsSwarm": null,
  "environment": { "...": "..." },
  "mounts": [
    {
      "mountId": "Of_sJqbowYSXSV6I-Rf5-",
      "type": "volume",
      "volumeName": "my-mariadb-ot1nlc-data",
      "mountPath": "/var/lib/mysql",
      "serviceType": "mariadb",
      "mariadbId": "SfJMoi2CiK0_03YiYg6hx"
    }
  ],
  "server": null,
  "backups": []
}
```

**Password in read:** Both `databasePassword` and `databaseRootPassword` ARE returned in plaintext by `mariadb.one`. Treat both as sensitive.

**`databaseRootPassword` note:** Auto-generated if not supplied on create (same as mysql).

**Default image caution:** The server's default image is `mariadb:6` which does not exist on Docker Hub. Always supply `dockerImage` on create (use `mariadb:11` or later).

---

### `POST /api/mariadb.create`

**Request body:**
```json
{
  "name": "my-mariadb",
  "appName": "my-mariadb",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "databaseName": "mydb",
  "databaseUser": "myuser",
  "databasePassword": "mypassword",
  "dockerImage": "mariadb:11"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (auto-suffixed) |
| `environmentId` | string | yes | Parent environment ID |
| `databaseName` | string | yes | Database name |
| `databaseUser` | string | yes | MariaDB user name |
| `databasePassword` | string | yes | MariaDB user password |
| `databaseRootPassword` | string | no | Root password — auto-generated if omitted |
| `dockerImage` | string | **strongly recommended** | Default `mariadb:6` is invalid; always pass `mariadb:11` or later |
| `description` | string | no | Optional description |

**Response:** `200 application/json` — the created mariadb object (includes auto-generated `databaseRootPassword`).

---

### `POST /api/mariadb.update`

**Request body:**
```json
{
  "mariadbId": "SfJMoi2CiK0_03YiYg6hx",
  "description": "updated",
  "externalPort": 3307,
  "dockerImage": "mariadb:11"
}
```

**Response:** `200 application/json` — `true`.

---

### `POST /api/mariadb.deploy`

**Request body:** `{ "mariadbId": "SfJMoi2CiK0_03YiYg6hx" }`

**Response:** `200 application/json` — full mariadb object. Poll until `applicationStatus` is `done` or `error`.

---

### `POST /api/mariadb.remove`

**Request body:** `{ "mariadbId": "SfJMoi2CiK0_03YiYg6hx" }`

**Response:** `200 application/json` — the deleted mariadb object.

---

## mongo.*

> Verified against live instance on 2026-05-22.

**Note on `mongo.all`:** Does NOT exist (returns 404). MongoDB instances are listed via `project.one` or `environment.one` (the `mongo` array).

**Schema difference:** MongoDB does NOT have a `databaseName` field. The create/one response only has `databaseUser` and `databasePassword` (plus `replicaSets`). This differs from the postgres/mysql/mariadb pattern.

### `GET /api/mongo.one?mongoId=<id>`

**Query params:** `mongoId` (string, required).

**Response:** `200 application/json`

```json
{
  "mongoId": "1ZU67MtuBacO2Su9_8J1s",
  "name": "my-mongo",
  "appName": "my-mongo-p4xnti",
  "description": null,
  "databaseUser": "myuser",
  "databasePassword": "mypassword",
  "replicaSets": false,
  "dockerImage": "mongo:7",
  "command": null,
  "args": null,
  "env": null,
  "externalPort": null,
  "memoryReservation": null,
  "memoryLimit": null,
  "cpuReservation": null,
  "cpuLimit": null,
  "replicas": 1,
  "applicationStatus": "done",
  "createdAt": "2026-05-23T01:32:59.627Z",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "serverId": null,
  "healthCheckSwarm": null,
  "restartPolicySwarm": null,
  "placementSwarm": null,
  "updateConfigSwarm": null,
  "rollbackConfigSwarm": null,
  "modeSwarm": null,
  "labelsSwarm": null,
  "networkSwarm": null,
  "stopGracePeriodSwarm": null,
  "endpointSpecSwarm": null,
  "ulimitsSwarm": null,
  "environment": { "...": "..." },
  "mounts": [
    {
      "mountId": "MVazoltO0jtbcbtn7uTNe",
      "type": "volume",
      "volumeName": "my-mongo-p4xnti-data",
      "mountPath": "/data/db",
      "serviceType": "mongo",
      "mongoId": "1ZU67MtuBacO2Su9_8J1s"
    }
  ],
  "server": null,
  "backups": []
}
```

**Password in read:** `databasePassword` IS returned in plaintext by `mongo.one`. Treat as sensitive.

**No `databaseName`:** MongoDB does not use a `databaseName` field. Do not include it in create/update bodies.

**Default image caution:** The server's default image is `mongo:15` which does not exist on Docker Hub. Always supply `dockerImage` on create (use `mongo:7` or `mongo:8`).

---

### `POST /api/mongo.create`

**Request body:**
```json
{
  "name": "my-mongo",
  "appName": "my-mongo",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "databaseUser": "myuser",
  "databasePassword": "mypassword",
  "dockerImage": "mongo:7"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (auto-suffixed) |
| `environmentId` | string | yes | Parent environment ID |
| `databaseUser` | string | yes | MongoDB admin user name |
| `databasePassword` | string | yes | MongoDB admin password |
| `replicaSets` | boolean | no | Enable replica sets — defaults to `false` |
| `dockerImage` | string | **strongly recommended** | Default `mongo:15` is invalid; always pass `mongo:7` or later |
| `description` | string | no | Optional description |

**Note:** There is no `databaseName` field for MongoDB — the auth database is always `admin` in this deployment model.

**Response:** `200 application/json` — the created mongo object.

---

### `POST /api/mongo.update`

**Request body:**
```json
{
  "mongoId": "1ZU67MtuBacO2Su9_8J1s",
  "description": "updated",
  "externalPort": 27018,
  "replicaSets": false
}
```

**Response:** `200 application/json` — `true`.

---

### `POST /api/mongo.deploy`

**Request body:** `{ "mongoId": "1ZU67MtuBacO2Su9_8J1s" }`

**Response:** `200 application/json` — full mongo object. Poll until `applicationStatus` is `done` or `error`.

---

### `POST /api/mongo.remove`

**Request body:** `{ "mongoId": "1ZU67MtuBacO2Su9_8J1s" }`

**Response:** `200 application/json` — the deleted mongo object.

---

## redis.*

> Verified against live instance on 2026-05-22.

**Note on `redis.all`:** Does NOT exist (returns 404). Redis instances are listed via `project.one` or `environment.one` (the `redis` array).

**Schema difference:** Redis only has `databasePassword` — there is no `databaseName` or `databaseUser`. This is the simplest database schema.

### `GET /api/redis.one?redisId=<id>`

**Query params:** `redisId` (string, required).

**Response:** `200 application/json`

```json
{
  "redisId": "l38B9KASb_1ILtjso1mzY",
  "name": "my-redis",
  "appName": "my-redis-w8npn1",
  "description": null,
  "databasePassword": "mypassword",
  "dockerImage": "redis:8",
  "command": null,
  "args": null,
  "env": null,
  "externalPort": null,
  "memoryReservation": null,
  "memoryLimit": null,
  "cpuReservation": null,
  "cpuLimit": null,
  "replicas": 1,
  "applicationStatus": "done",
  "createdAt": "2026-05-23T01:33:00.123Z",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "serverId": null,
  "healthCheckSwarm": null,
  "restartPolicySwarm": null,
  "placementSwarm": null,
  "updateConfigSwarm": null,
  "rollbackConfigSwarm": null,
  "modeSwarm": null,
  "labelsSwarm": null,
  "networkSwarm": null,
  "stopGracePeriodSwarm": null,
  "endpointSpecSwarm": null,
  "ulimitsSwarm": null,
  "environment": { "...": "..." },
  "mounts": [
    {
      "mountId": "7JnmG9wFbuyM9j7AbFs_q",
      "type": "volume",
      "volumeName": "my-redis-w8npn1-data",
      "mountPath": "/data",
      "serviceType": "redis",
      "redisId": "l38B9KASb_1ILtjso1mzY"
    }
  ],
  "server": null
}
```

**Note:** The redis object does NOT have a `backups` field (unlike postgres/mysql/mariadb/mongo which all include `"backups": []`).

**Password in read:** `databasePassword` IS returned in plaintext by `redis.one`. Treat as sensitive.

---

### `POST /api/redis.create`

**Request body:**
```json
{
  "name": "my-redis",
  "appName": "my-redis",
  "environmentId": "3syo_vjPnl-5xjNjCFjV_",
  "databasePassword": "mypassword"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker service name (auto-suffixed) |
| `environmentId` | string | yes | Parent environment ID |
| `databasePassword` | string | yes | Redis AUTH password |
| `dockerImage` | string | no | Defaults to `redis:8` |
| `description` | string | no | Optional description |

**Note:** There is no `databaseName` or `databaseUser` for Redis.

**Response:** `200 application/json` — the created redis object.

---

### `POST /api/redis.update`

**Request body:**
```json
{
  "redisId": "l38B9KASb_1ILtjso1mzY",
  "description": "updated",
  "externalPort": 6380
}
```

**Response:** `200 application/json` — `true`.

---

### `POST /api/redis.deploy`

**Request body:** `{ "redisId": "l38B9KASb_1ILtjso1mzY" }`

**Response:** `200 application/json` — full redis object (same as `redis.one`). Poll until `applicationStatus` is `done` or `error`.

---

### `POST /api/redis.remove`

**Request body:** `{ "redisId": "l38B9KASb_1ILtjso1mzY" }`

**Response:** `200 application/json` — the deleted redis object.

---

## Deployment Status Reference

### `applicationStatus` field (on application objects)

Values observed on live instance:

| Value | Meaning |
|-------|---------|
| `idle` | Application exists but has never been deployed |
| `running` | Deploy in progress (transient state) |
| `done` | Last deployment completed successfully |
| `error` | Last deployment failed (inferred from deployment record; not directly observed on `applicationStatus` field — see note) |
| `stopped` | Application is stopped (not verified — assumed from Dokploy source) |

**Observed transitions during this probe session:**
1. After `application.create`: `idle`
2. Immediately after `application.deploy`: `running`
3. ~5 seconds after `application.deploy` (nginx:alpine image): `done`

**Database-specific transitions (verified 2026-05-22):**
- After `postgres.create`, `mysql.create`, `mariadb.create`, `mongo.create`, `redis.create`: `idle`
- After `<db>.deploy` is called: status may still show `idle` (deploy is enqueued asynchronously); poll with `<db>.one`
- After deploy completes successfully: `done`
- After deploy fails (e.g. invalid Docker image): `error` transiently, then transitions to `done` after successful redeploy
- `running` observed for mysql (heavier image, slower pull) immediately after deploy call

**Poll strategy for databases:** Same as applications — poll `<db>.one` on `applicationStatus`; stop when value is `done` or `error`.

### Deployment record `status` field (inside `deployments[]` on `application.one`)

Values observed on live deployments:

| Value | Meaning |
|-------|---------|
| `done` | Deployment completed successfully |
| `error` | Deployment failed |
| `running` | Deployment in progress (inferred) |

**Finished states (polling can stop):** `done`, `error`  
**In-progress states (polling should continue):** `running`, `idle` (if waiting for first deploy)

---

## Error Response Shape

All validation and not-found errors follow this shape:

```json
{
  "message": "Input validation failed",
  "code": "BAD_REQUEST",
  "data": {
    "code": "BAD_REQUEST",
    "httpStatus": 400,
    "path": "application.saveEnvironment",
    "zodError": {
      "formErrors": [],
      "fieldErrors": {
        "buildArgs": ["Invalid input: expected nonoptional, received undefined"]
      }
    }
  },
  "issues": [
    {
      "code": "invalid_type",
      "expected": "nonoptional",
      "path": ["buildArgs"],
      "message": "Invalid input: expected nonoptional, received undefined"
    }
  ]
}
```

HTTP status codes: `200` success, `400` bad request / validation error, `401` unauthorized, `404` not found.

---

## Risk Item Answers

### Risk Item 1: Does `application.one` return `registryPassword`?

**Answer: No.** There is no `registryPassword` field. Docker private registry credentials are stored in two separate fields:
- `username` (string|null)
- `password` (string|null)

Both are returned by `application.one` and are `null` when no private registry is used. For the `saveDockerProvider` call, pass both as `null` for public images.

### Risk Item 2: Does the `environment.*` router exist?

**Answer: Yes, confirmed.** All four CRUD methods exist:
- `GET /api/environment.one?environmentId=<id>`
- `POST /api/environment.create` — body: `{ projectId, name, description? }`
- `POST /api/environment.update` — body: `{ environmentId, name?, description?, env? }`
- `POST /api/environment.remove` — body: `{ environmentId }`

---

## Appendix: `sourceType` Values

The `sourceType` field on an application controls which git/registry source is active:

| Value | Source |
|-------|--------|
| `github` | GitHub App integration |
| `gitlab` | GitLab integration |
| `gitea` | Gitea integration |
| `bitbucket` | Bitbucket integration |
| `docker` | Docker image (public or private registry) |
| `git` | Custom git URL |
| `drop` | File drop/upload |

For the Terraform provider v1, only `docker` is supported (simplest case, no OAuth setup required).
