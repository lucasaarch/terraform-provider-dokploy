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

---

## destination.*

> Verified against live instance on 2026-05-23.

S3-compatible storage destinations live at the **organization** level (not project or environment). Each destination is identified by `destinationId`.

### `GET /api/destination.all`

Returns all destinations in the organization that owns the API key.

**Request:** no body, no query params.

**Response:** `200 application/json` — array of destination objects.

```json
[
  {
    "destinationId": "FwQFgPCZe4wKraiAd_dyd",
    "name": "blitz-backups",
    "provider": "DigitalOcean",
    "accessKey": "AKIAEXAMPLEKEY1234",
    "secretAccessKey": "ExampleSecretKey",
    "bucket": "my-backups",
    "region": "nyc3",
    "endpoint": "https://nyc3.digitaloceanspaces.com",
    "additionalFlags": [],
    "organizationId": "BTFAI_7TzbiGeXtbPMTT-",
    "createdAt": "2026-04-08T04:29:05.237Z"
  }
]
```

---

### `GET /api/destination.one?destinationId=<id>`

Returns a single destination.

**Query params:** `destinationId` (string, required).

**Response:** `200 application/json` — single destination object (same shape as array item above).

**Error:** Returns `401 UNAUTHORIZED` if the API key does not have access to the destination's organization.

---

### `POST /api/destination.create`

Creates a new destination.

**Request body:**

```json
{
  "name": "prod-backups",
  "provider": "DigitalOcean",
  "bucket": "my-bucket",
  "region": "nyc3",
  "endpoint": "https://nyc3.digitaloceanspaces.com",
  "accessKey": "AKIAEXAMPLEKEY1234",
  "secretAccessKey": "ExampleSecretKey"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `provider` | string | yes | S3 provider name — **free string, no server-side enum validation**. Observed values: `"DigitalOcean"`, `"AWS"`, etc. |
| `bucket` | string | yes | S3 bucket name |
| `region` | string | yes | S3 region (pass `""` for providers that don't use it) |
| `endpoint` | string | yes | S3 endpoint URL |
| `accessKey` | string | yes | S3 access key ID |
| `secretAccessKey` | string | yes | S3 secret access key |
| `additionalFlags` | []string | no | Extra flags forwarded to the backup tool |

**Response:** `200 application/json` — the created destination object.

**Note on `provider`:** The server accepts any string value for `provider` — Zod validates it only as `nonoptional`. The Terraform provider should use `provider_type` as the schema attribute name (to avoid conflicting with Terraform's `provider` meta-argument) and document the observed values.

---

### `POST /api/destination.update`

Updates a destination. **All create fields are required** (not a partial update).

**Request body:**

```json
{
  "destinationId": "FwQFgPCZe4wKraiAd_dyd",
  "name": "prod-backups-renamed",
  "provider": "DigitalOcean",
  "bucket": "my-bucket",
  "region": "nyc3",
  "endpoint": "https://nyc3.digitaloceanspaces.com",
  "accessKey": "AKIAEXAMPLEKEY1234",
  "secretAccessKey": "ExampleSecretKey"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `destinationId` | string | yes | ID of destination to update |
| `name` | string | yes | Display name |
| `provider` | string | yes | Provider string (free-form, see `destination.create`) |
| `bucket` | string | yes | Bucket name |
| `region` | string | yes | Region |
| `endpoint` | string | yes | Endpoint URL |
| `accessKey` | string | yes | Access key |
| `secretAccessKey` | string | yes | Secret key |
| `additionalFlags` | []string | no | Extra flags |

**Response:** `200 application/json` — the updated destination object.

---

### `POST /api/destination.remove`

Deletes a destination. **Endpoint is `.remove`, not `.delete`** (`.delete` returns 404).

**Request body:**

```json
{
  "destinationId": "FwQFgPCZe4wKraiAd_dyd"
}
```

**Response:** `200 application/json` — the deleted destination object.

---

## backup.*

> Verified against live instance on 2026-05-23.

Backups attach to a database or application resource via both a generic `database` field (containing the resource ID) and a typed ID field (`postgresId`, `mysqlId`, etc.).

**Critical behavior: `backup.create` requires the typed ID field to persist.** Sending only `database` + `databaseType` returns HTTP 200 with an empty body and the backup is silently discarded. You must also send `postgresId`, `mysqlId`, `mariadbId`, `mongoId`, or `libsqlId` alongside `database` for the backup to actually be created.

**`backup.all` does NOT exist** (returns 404). Backups are listed via the parent resource's `.one` endpoint (e.g. `postgres.one` returns `backups[]` on the postgres object).

---

### `GET /api/backup.one?backupId=<id>`

Returns a single backup with full details including related destination and database objects.

**Query params:** `backupId` (string, required).

**Response:** `200 application/json`

```json
{
  "backupId": "vFoXPdHeTdR3S-tll0mhM",
  "appName": "backup-back-up-cross-platform-driver-3tckyf",
  "schedule": "0 3 * * *",
  "enabled": null,
  "database": "1W24xWRZsPqg-iGWOPTdA",
  "prefix": "tf-probe/",
  "serviceName": null,
  "destinationId": "Fg1H5b0lhIwvaj4je8tlb",
  "keepLatestCount": null,
  "backupType": "database",
  "databaseType": "postgres",
  "composeId": null,
  "postgresId": "1W24xWRZsPqg-iGWOPTdA",
  "mariadbId": null,
  "mysqlId": null,
  "mongoId": null,
  "libsqlId": null,
  "userId": null,
  "metadata": null,
  "postgres": { "...": "full postgres object" },
  "mysql": null,
  "mariadb": null,
  "mongo": null,
  "libsql": null,
  "destination": { "...": "full destination object" },
  "compose": null
}
```

**Complete field list on backup object:**

| Field | Type | Notes |
|-------|------|-------|
| `backupId` | string | Primary key |
| `appName` | string | Auto-generated internal name |
| `schedule` | string | Cron expression |
| `enabled` | boolean\|null | Whether the backup is active; null = not explicitly set (defaults to enabled behavior) |
| `database` | string | ID of the resource being backed up (same as the typed ID field below) |
| `prefix` | string | Path prefix inside the bucket |
| `serviceName` | string\|null | Internal service name (null for database backups) |
| `destinationId` | string | Destination ID |
| `keepLatestCount` | integer\|null | Retention count (null = keep all) |
| `backupType` | string | `"database"` or `"compose"` |
| `databaseType` | string | See enum below |
| `composeId` | string\|null | Set when `backupType` is `"compose"` |
| `postgresId` | string\|null | Set when `databaseType` is `"postgres"` |
| `mariadbId` | string\|null | Set when `databaseType` is `"mariadb"` |
| `mysqlId` | string\|null | Set when `databaseType` is `"mysql"` |
| `mongoId` | string\|null | Set when `databaseType` is `"mongo"` |
| `libsqlId` | string\|null | Set when `databaseType` is `"libsql"` |
| `userId` | string\|null | Owner user ID |
| `metadata` | any\|null | Opaque metadata blob |
| `postgres` | object\|null | Embedded postgres object |
| `mysql` | object\|null | Embedded mysql object |
| `mariadb` | object\|null | Embedded mariadb object |
| `mongo` | object\|null | Embedded mongo object |
| `libsql` | object\|null | Embedded libsql object |
| `destination` | object\|null | Embedded destination object |
| `compose` | object\|null | Embedded compose object |
| `deployments` | array | Backup run history |

---

### `POST /api/backup.create`

Creates a scheduled backup.

**Request body:**

```json
{
  "schedule": "0 3 * * *",
  "prefix": "postgres/app/",
  "destinationId": "FwQFgPCZe4wKraiAd_dyd",
  "database": "1W24xWRZsPqg-iGWOPTdA",
  "databaseType": "postgres",
  "postgresId": "1W24xWRZsPqg-iGWOPTdA"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `schedule` | string | yes | Cron expression |
| `prefix` | string | yes | Bucket path prefix |
| `destinationId` | string | yes | Destination ID |
| `database` | string | yes | Resource ID being backed up |
| `databaseType` | string | yes | Enum: `"postgres"` \| `"mariadb"` \| `"mysql"` \| `"mongo"` \| `"web-server"` \| `"libsql"` |
| `postgresId` | string | **required when databaseType is `postgres`** | Must match `database`; backup silently discarded without it |
| `mysqlId` | string | **required when databaseType is `mysql`** | Must match `database` |
| `mariadbId` | string | **required when databaseType is `mariadb`** | Must match `database` |
| `mongoId` | string | **required when databaseType is `mongo`** | Must match `database` |
| `libsqlId` | string | **required when databaseType is `libsql`** | Must match `database` |
| `enabled` | boolean\|null | no | Whether enabled; null = server default (active) |
| `keepLatestCount` | integer\|null | no | Retention count |
| `serviceName` | string\|null | no | Internal override |
| `metadata` | any\|null | no | Opaque metadata |

**Response:** `200 application/json` — **empty body**. The backup ID must be obtained from the parent resource's `.one` endpoint after creation (look at `backups[]`).

**Warning:** If the typed ID field (`postgresId` etc.) is omitted, the API returns HTTP 200 with empty body and the backup is NOT persisted. Always include both `database` and the typed ID field.

---

### `POST /api/backup.update`

Updates a backup. **All fields are required** (not a partial update).

**Request body:**

```json
{
  "backupId": "vFoXPdHeTdR3S-tll0mhM",
  "schedule": "0 4 * * *",
  "prefix": "postgres/app/",
  "destinationId": "FwQFgPCZe4wKraiAd_dyd",
  "database": "1W24xWRZsPqg-iGWOPTdA",
  "databaseType": "postgres",
  "enabled": true,
  "keepLatestCount": null,
  "serviceName": null,
  "metadata": null
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `backupId` | string | yes | Backup ID |
| `schedule` | string | yes (via `prefix`/`destinationId`/etc.) | — |
| `prefix` | string | yes | — |
| `destinationId` | string | yes | — |
| `database` | string | yes | — |
| `databaseType` | string | yes | Enum: `"postgres"` \| `"mariadb"` \| `"mysql"` \| `"mongo"` \| `"web-server"` \| `"libsql"` |
| `enabled` | boolean\|null | yes (nonoptional) | Pass `true`/`false`/`null` |
| `keepLatestCount` | integer\|null | yes (nonoptional) | Pass `null` to keep all |
| `serviceName` | string\|null | yes (nonoptional) | Pass `null` if unused |
| `metadata` | any\|null | yes (nonoptional) | Pass `null` if unused |

**Response:** `200 application/json` — **empty body**.

---

### `POST /api/backup.remove`

Deletes a backup. **Endpoint is `.remove`, not `.delete`** (`.delete` returns 404).

**Request body:**

```json
{
  "backupId": "vFoXPdHeTdR3S-tll0mhM"
}
```

**Response:** `200 application/json` — the deleted backup object (without relation sub-objects).

---

## schedule.*

> Verified against live instance on 2026-05-23.

Schedules are cron commands that run on a target (application container, compose service, remote server, or the Dokploy host itself). Each schedule gets an auto-generated `appName`.

---

### `GET /api/schedule.one?scheduleId=<id>`

Returns a single schedule with its related application/compose/server objects.

**Query params:** `scheduleId` (string, required).

**Response:** `200 application/json`

```json
{
  "scheduleId": "gFox20q7bMnZyJGfU-vvM",
  "name": "warmup-cache",
  "cronExpression": "*/15 * * * *",
  "appName": "schedule-parse-solid-state-program-61hjaf",
  "serviceName": null,
  "shellType": "bash",
  "scheduleType": "application",
  "command": "curl -s http://localhost:3000/internal/warmup",
  "script": null,
  "applicationId": null,
  "composeId": null,
  "serverId": null,
  "userId": null,
  "enabled": true,
  "timezone": null,
  "createdAt": "2026-05-23T02:34:46.364Z",
  "application": null,
  "compose": null,
  "server": null
}
```

**Complete field list on schedule object:**

| Field | Type | Notes |
|-------|------|-------|
| `scheduleId` | string | Primary key |
| `name` | string | Display name |
| `cronExpression` | string | Cron schedule |
| `appName` | string | Auto-generated internal name (computed on create) |
| `serviceName` | string\|null | Internal service name override |
| `shellType` | string | Shell: `"bash"` (default), `"sh"`, etc. |
| `scheduleType` | string | See enum below |
| `command` | string | Shell command to execute |
| `script` | string\|null | Script body (alternative to `command`) |
| `applicationId` | string\|null | Set when `scheduleType` is `"application"` |
| `composeId` | string\|null | Set when `scheduleType` is `"compose"` |
| `serverId` | string\|null | Set when `scheduleType` is `"server"` |
| `userId` | string\|null | Owner user ID |
| `enabled` | boolean | Whether the schedule is active (defaults to `true` on create) |
| `timezone` | string\|null | IANA timezone string; null = UTC |
| `createdAt` | string | ISO 8601 timestamp |
| `application` | object\|null | Embedded application object |
| `compose` | object\|null | Embedded compose object |
| `server` | object\|null | Embedded server object |

**`scheduleType` enum:**

| Value | Target |
|-------|--------|
| `application` | Run command inside an application container |
| `compose` | Run command inside a compose service |
| `server` | Run command on a remote server |
| `dokploy-server` | Run command on the Dokploy host itself |

---

### `POST /api/schedule.create`

Creates a scheduled cron command.

**Request body:**

```json
{
  "name": "warmup-cache",
  "cronExpression": "*/15 * * * *",
  "command": "curl -s http://localhost:3000/internal/warmup",
  "scheduleType": "application",
  "applicationId": "Y2gQJgGGT5wBmaEZ35blK"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `cronExpression` | string | yes | Cron expression |
| `command` | string | yes | Shell command to run |
| `scheduleType` | string | no | Enum: `"application"` \| `"compose"` \| `"server"` \| `"dokploy-server"`. Defaults to `"application"` if omitted |
| `shellType` | string | no | Shell type; defaults to `"bash"` |
| `applicationId` | string | required if `scheduleType` = `"application"` | Application ID |
| `composeId` | string | required if `scheduleType` = `"compose"` | Compose stack ID |
| `serverId` | string | required if `scheduleType` = `"server"` | Server ID |
| `enabled` | boolean | no | Defaults to `true` |
| `timezone` | string | no | IANA timezone (e.g. `"America/Sao_Paulo"`); null = UTC |

**Response:** `200 application/json` — the created schedule object (same shape as `schedule.one` minus relation sub-objects).

---

### `POST /api/schedule.update`

Updates a schedule. **`name` and `cronExpression` are required**; other fields are optional.

**Request body:**

```json
{
  "scheduleId": "gFox20q7bMnZyJGfU-vvM",
  "name": "warmup-cache",
  "cronExpression": "*/15 * * * *",
  "command": "echo updated"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `scheduleId` | string | yes | Schedule ID |
| `name` | string | yes | Display name |
| `cronExpression` | string | yes | Cron expression |
| `command` | string | no | New command |
| `shellType` | string | no | New shell type |
| `enabled` | boolean | no | Toggle active state |
| `timezone` | string | no | New timezone |

**Response:** `200 application/json` — the updated schedule object.

---

### `POST /api/schedule.delete`

Deletes a schedule. **Endpoint is `.delete`** (confirmed working; unlike backup/destination which use `.remove`).

**Request body:**

```json
{
  "scheduleId": "gFox20q7bMnZyJGfU-vvM"
}
```

**Response:** `200 application/json` — `true` (boolean literal).

---

## sshKey.*

> Verified against live instance on 2026-05-23.

SSH keys are registered at the **organization** level. Each key is identified by `sshKeyId`. Keys are used by `server.*` to authenticate SSH connections to remote machines.

**Critical behavior: `sshKey.create` returns an empty body.** The created key's ID must be obtained from `sshKey.all` after creation (match by `name` + `createdAt`). This differs from most other create endpoints which return the created object.

---

### `GET /api/sshKey.all`

Returns all SSH keys visible to the authenticated API key (across all accessible organizations).

**Request:** no body, no query params.

**Response:** `200 application/json` — array of SSH key objects.

```json
[
  {
    "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
    "name": "my-deploy-key",
    "privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----\n",
    "publicKey": "ssh-rsa AAAAB3NzaC1yc2EAAA... user@host\n",
    "description": null,
    "createdAt": "2026-05-23T03:58:03.531Z",
    "lastUsedAt": null,
    "organizationId": "JYzDaUdW-hC0EX785HuXV"
  }
]
```

---

### `GET /api/sshKey.one?sshKeyId=<id>`

Returns a single SSH key with full details.

**Query params:** `sshKeyId` (string, required).

**Response:** `200 application/json`

```json
{
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
  "name": "my-deploy-key",
  "privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----\n",
  "publicKey": "ssh-rsa AAAAB3NzaC1yc2EAAA... user@host\n",
  "description": null,
  "createdAt": "2026-05-23T03:58:03.531Z",
  "lastUsedAt": null,
  "organizationId": "JYzDaUdW-hC0EX785HuXV"
}
```

**Complete field list:**

| Field | Type | Notes |
|-------|------|-------|
| `sshKeyId` | string | Primary key |
| `name` | string | Display name |
| `privateKey` | string | PEM-encoded private key — **returned in plaintext** |
| `publicKey` | string | OpenSSH-format public key |
| `description` | string\|null | Optional description |
| `createdAt` | string | ISO 8601 timestamp |
| `lastUsedAt` | string\|null | Last use timestamp (null if unused) |
| `organizationId` | string | Owning organization ID |

**Private key in read:** `privateKey` IS returned in plaintext by `sshKey.one` (and by `sshKey.all`). The Terraform provider Read method should overwrite the state value from the API response — no need to preserve the plan value (unlike `registry_password`). Mark the field as `Sensitive: true` in the schema.

---

### `POST /api/sshKey.create`

Creates a new SSH key.

**Request body:**
```json
{
  "name": "my-deploy-key",
  "organizationId": "JYzDaUdW-hC0EX785HuXV",
  "publicKey": "ssh-rsa AAAAB3NzaC1yc2EAAA... user@host\n",
  "privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----\n"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `organizationId` | string | yes | Organization that will own the key |
| `publicKey` | string | yes | OpenSSH-format public key |
| `privateKey` | string | yes | PEM-encoded private key |

**Response:** `200 application/json` — **empty body**. The created key is NOT returned inline; retrieve it from `sshKey.all` (match by `name` + `organizationId` + `createdAt`).

**Warning:** This differs from other create endpoints. Callers must use `sshKey.all` after create to obtain the `sshKeyId`.

---

### `POST /api/sshKey.update`

Updates an SSH key's name and/or description. **`name` is required alongside `sshKeyId`.**

**Request body:**
```json
{
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
  "name": "my-deploy-key-renamed",
  "description": "optional description"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `sshKeyId` | string | yes | SSH key ID |
| `name` | string | yes | New display name (required — omitting causes a backend error even though Zod passes) |
| `description` | string | no | New description |

**Note on `privateKey`/`publicKey` updates:** Sending `privateKey` or `publicKey` fields passes Zod validation but is rejected by the backend with HTTP 400 `"Error updating this SSH key"` (`zodError: null`). These fields cannot be changed via `sshKey.update` — to rotate keys, delete and recreate the resource.

**Response:** `200 application/json` — the full updated SSH key object (same shape as `sshKey.one`).

---

### `POST /api/sshKey.remove`

Deletes an SSH key. **Endpoint is `.remove`, not `.delete`** (`.delete` returns 404).

**Request body:**
```json
{
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY"
}
```

**Response:** `200 application/json` — the deleted SSH key object (same shape as `sshKey.one`).

---

## server.*

> Verified against live instance on 2026-05-23.

Servers are remote machines registered as managed workers in Dokploy. They are managed at the **organization** level (inferred from the SSH key used). Each server is identified by `serverId`.

**SSH handshake behavior:** `server.create` returns HTTP 200 with the full server object immediately — the record is created synchronously. The actual SSH connectivity test runs asynchronously in the background. The `serverStatus` field reflects the connection state after the handshake completes.

---

### `GET /api/server.all`

Returns all servers visible to the authenticated API key.

**Request:** no body, no query params.

**Response:** `200 application/json` — array of server objects (same shape as `server.one` without `deployments` and `sshKey` sub-objects).

---

### `GET /api/server.one?serverId=<id>`

Returns a single server with full details.

**Query params:** `serverId` (string, required).

**Response:** `200 application/json`

```json
{
  "serverId": "7xq6PuwCfVaQ4tHNRYXUL",
  "name": "my-worker",
  "description": "",
  "ipAddress": "1.2.3.4",
  "port": 22,
  "username": "root",
  "appName": "server-compress-primary-microchip-9bcanv",
  "enableDockerCleanup": false,
  "createdAt": "2026-05-23T03:59:24.683Z",
  "organizationId": "JYzDaUdW-hC0EX785HuXV",
  "serverStatus": "active",
  "serverType": "deploy",
  "command": "",
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
  "metricsConfig": {
    "server": {
      "port": 4500,
      "type": "Remote",
      "token": "",
      "cronJob": "",
      "thresholds": { "cpu": 0, "memory": 0 },
      "refreshRate": 60,
      "urlCallback": "",
      "retentionDays": 2
    },
    "containers": {
      "services": { "exclude": [], "include": [] },
      "refreshRate": 60
    }
  },
  "deployments": [],
  "sshKey": {
    "sshKeyId": "...",
    "privateKey": "...",
    "publicKey": "...",
    "name": "...",
    "description": null,
    "createdAt": "...",
    "lastUsedAt": null,
    "organizationId": "..."
  }
}
```

**Complete top-level field list on server object:**

| Field | Type | Notes |
|-------|------|-------|
| `serverId` | string | Primary key |
| `name` | string | Display name |
| `description` | string | Description (empty string `""` is valid; NOT nullable — always present) |
| `ipAddress` | string | IP or hostname |
| `port` | integer | SSH port (typically 22) |
| `username` | string | SSH username |
| `appName` | string | Auto-generated internal name |
| `enableDockerCleanup` | boolean | Whether Docker cleanup is enabled (defaults to `false`) |
| `createdAt` | string | ISO 8601 timestamp |
| `organizationId` | string | Owning organization (inferred from `sshKeyId` on create) |
| `serverStatus` | string | Connectivity status: `"active"`, `"inactive"`, `"error"` |
| `serverType` | string | `"deploy"` (runs workloads) or `"build"` (used as a build host) |
| `command` | string | Custom setup command (empty string by default) |
| `sshKeyId` | string | SSH key ID used for authentication |
| `metricsConfig` | object | Metrics collection configuration |
| `deployments` | array | Deployment history (only on `server.one`) |
| `sshKey` | object | Embedded SSH key object (only on `server.one`; includes `privateKey` in plaintext) |

---

### `POST /api/server.create`

Creates a new remote server record. The SSH handshake runs asynchronously after the record is created.

**Request body:**
```json
{
  "name": "my-worker",
  "description": "",
  "ipAddress": "1.2.3.4",
  "port": 22,
  "username": "root",
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
  "serverType": "deploy"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `description` | string | yes | Description — **nonoptional** (Zod requires it present; pass `""` for empty) |
| `ipAddress` | string | yes | IP or hostname |
| `port` | number | yes | SSH port |
| `username` | string | yes | SSH username (nonoptional) |
| `sshKeyId` | string | yes | SSH key ID (nonoptional) |
| `serverType` | string | yes | `"deploy"` or `"build"` (nonoptional) |

**Note:** `organizationId` is NOT required in the body — it is inferred server-side from the SSH key's organization.

**Response:** `200 application/json` — the full created server object (same shape as `server.one` minus `deployments` and `sshKey` sub-objects). The `serverStatus` is `"active"` immediately on create; the async SSH test may change it later.

---

### `POST /api/server.update`

Updates a server record. **All create fields are required** (not a partial update).

**Request body:**
```json
{
  "serverId": "7xq6PuwCfVaQ4tHNRYXUL",
  "name": "my-worker-renamed",
  "description": "",
  "ipAddress": "1.2.3.4",
  "port": 22,
  "username": "root",
  "sshKeyId": "0Y7QbwR0-NYV2cjREsPYY",
  "serverType": "deploy"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `serverId` | string | yes | Server ID (nonoptional) |
| `name` | string | yes | Display name |
| `description` | string | yes | Description (nonoptional — pass `""` for empty) |
| `ipAddress` | string | yes | IP or hostname |
| `port` | number | yes | SSH port |
| `username` | string | yes | SSH username (nonoptional) |
| `sshKeyId` | string | yes | SSH key ID (nonoptional) |
| `serverType` | string | yes | `"deploy"` or `"build"` (nonoptional) |

**Response:** HTTP status unknown at probe time — returns the updated server object or `true`.

---

### `POST /api/server.remove`

Deletes a server. **Endpoint is `.remove`, not `.delete`** (`.delete` returns 404).

**Request body:**
```json
{
  "serverId": "7xq6PuwCfVaQ4tHNRYXUL"
}
```

**Response:** `200 application/json` — the deleted server object.

---

## server_id field on existing routers

> Verified against live instance on 2026-05-23.

All five database routers accept `serverId` as an optional field on both `create` and (by extension) `update`. When `serverId` is provided, the database service is deployed onto the specified remote server rather than the Dokploy host.

### `serverId` on DB create endpoints

The probe sent `{"name":"tf-probe","appName":"tf-probe","environmentId":"nope","dockerImage":"none","serverId":"nope"}` to each DB create endpoint and inspected the Zod validation error. In every case, `serverId` did NOT appear in `fieldErrors` — confirming Zod accepts the field. The `fieldErrors` only complained about missing DB-specific required fields.

| Router | `serverId` accepted on create? | Zod errors on probe body |
|--------|-------------------------------|--------------------------|
| `postgres.create` | **yes** | `databaseName`, `databaseUser`, `databasePassword` |
| `mysql.create` | **yes** | `databaseName`, `databaseUser`, `databasePassword` |
| `mariadb.create` | **yes** | `databaseName`, `databaseUser`, `databasePassword` |
| `mongo.create` | **yes** | `databaseUser`, `databasePassword` |
| `redis.create` | **yes** | `databasePassword` |

**All 5 database routers accept `serverId`.** The field is present on the `*.one` responses as `"serverId": null` when unused (confirmed in v0.3 probes — see DB sections above).

**Implementation note:** Add an Optional `server_id` attribute (ForceNew) to each of the five database resource schemas. The field maps to `serverId` in the create/update body. Since the DB `*.one` endpoints already return `serverId`, the Read method can populate it from the API response without special handling.

---

## compose.*

> Verified against live instance on 2026-05-23.

Docker Compose stacks managed by Dokploy. Stacks use `composeId` as their primary key and `composeStatus` (NOT `applicationStatus`) as their deployment status field.

**Important differences from application:**
- Status field is `composeStatus`, not `applicationStatus`.
- Default `sourceType` on create is `"github"` — must be overridden to `"raw"` for inline YAML.
- `compose.update` returns the **full updated compose object** (not `true`).
- `compose.deploy` returns `{"success": true, "message": "Deployment queued", "composeId": "..."}` (not an empty body).
- Delete verb is `compose.delete` (not `compose.remove` — `.remove` returns 404).
- `compose.one` includes `backups: []` and `mounts: []` arrays.

---

### `GET /api/compose.one?composeId=<id>`

Returns a single compose stack with full details.

**Query params:** `composeId` (string, required).

**Response:** `200 application/json` — full compose object.

```json
{
  "composeId": "tKexKC4qIBasakvFUS2QY",
  "name": "tf-probe-compose",
  "appName": "tf-probe-compose-m1wg2s",
  "description": null,
  "env": "TEST_VAR=hello",
  "composeFile": "version: \"3\"\nservices:\n  hello:\n    image: nginx:alpine\n",
  "refreshToken": "J3EILVMLQMg-vYbDiubRi",
  "sourceType": "raw",
  "composeType": "docker-compose",
  "repository": null,
  "owner": null,
  "branch": null,
  "autoDeploy": true,
  "composePath": "./docker-compose.yml",
  "suffix": "",
  "randomize": false,
  "isolatedDeployment": false,
  "isolatedDeploymentsVolume": false,
  "triggerType": "push",
  "composeStatus": "done",
  "environmentId": "vPN6IgAMIeZjk0Fh1288H",
  "createdAt": "2026-05-23T04:51:13.021Z",
  "serverId": null,
  "environment": { "...": "..." },
  "backups": [],
  "mounts": [],
  "domains": [],
  "deployments": [],
  "server": null
}
```

**Complete key list on compose object:**
`appName`, `autoDeploy`, `backups`, `bitbucket`, `bitbucketBranch`, `bitbucketId`, `bitbucketOwner`, `bitbucketRepository`, `bitbucketRepositorySlug`, `branch`, `command`, `composeFile`, `composeId`, `composePath`, `composeStatus`, `composeType`, `createdAt`, `customGitBranch`, `customGitSSHKeyId`, `customGitUrl`, `deployments`, `description`, `domains`, `enableSubmodules`, `env`, `environment`, `environmentId`, `gitea`, `giteaBranch`, `giteaId`, `giteaOwner`, `giteaRepository`, `github`, `githubId`, `gitlab`, `gitlabBranch`, `gitlabId`, `gitlabOwner`, `gitlabPathNamespace`, `gitlabProjectId`, `gitlabRepository`, `hasGitProviderAccess`, `isolatedDeployment`, `isolatedDeploymentsVolume`, `mounts`, `name`, `owner`, `randomize`, `refreshToken`, `repository`, `server`, `serverId`, `sourceType`, `suffix`, `triggerType`, `unauthorizedProvider`, `watchPaths`

**`composeStatus` values:** Same as `applicationStatus` — `idle`, `running`, `done`, `error`, `stopped`.

---

### `POST /api/compose.create`

Creates a new compose stack.

**Request body:**
```json
{
  "name": "my-stack",
  "appName": "my-stack",
  "environmentId": "vPN6IgAMIeZjk0Fh1288H"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `appName` | string | yes | Docker stack name (auto-suffixed with random chars, e.g. `my-stack-m1wg2s`) |
| `environmentId` | string | yes | Parent environment ID |
| `description` | string | no | Optional description |
| `serverId` | string | no | Deploy on a remote server; omit for Dokploy host |

**Response:** `200 application/json` — the full compose object. `sourceType` will be `"github"` by default; `composeFile` will be `""`.

**Note:** After create, always call `compose.update` to set `sourceType: "raw"` and `composeFile` before deploying.

---

### `POST /api/compose.update`

Updates a compose stack's configuration.

**Request body:**
```json
{
  "composeId": "tKexKC4qIBasakvFUS2QY",
  "name": "my-stack",
  "sourceType": "raw",
  "composeFile": "version: \"3\"\nservices:\n  hello:\n    image: nginx:alpine\n",
  "env": "FOO=bar\nBAZ=qux"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `composeId` | string | yes | Compose stack ID |
| `name` | string | no | New display name |
| `description` | string | no | New description |
| `sourceType` | string | no | `"raw"` for inline YAML; `"github"`, `"gitlab"`, etc. for git sources |
| `composeFile` | string | no | Inline YAML content (when `sourceType = "raw"`) |
| `env` | string | no | Env vars (newline-separated KEY=value pairs) |

**Response:** `200 application/json` — the **full updated compose object** (unlike `application.update` which returns `true`).

---

### `POST /api/compose.deploy`

Triggers an asynchronous deployment of the stack.

**Request body:**
```json
{
  "composeId": "tKexKC4qIBasakvFUS2QY"
}
```

**Response:** `200 application/json`
```json
{"success": true, "message": "Deployment queued", "composeId": "tKexKC4qIBasakvFUS2QY"}
```

Poll `compose.one` until `composeStatus` is `done` or `error`.

---

### `POST /api/compose.delete`

Deletes a compose stack. **Endpoint is `.delete`** (`.remove` returns 404).

**Request body:**
```json
{
  "composeId": "tKexKC4qIBasakvFUS2QY"
}
```

**Response:** `200 application/json` — the deleted compose object.

---

## mounts.*

> Verified against live instance on 2026-05-23.

Mounts (bind, volume, or file) can be attached to any Dokploy service type. The router uses **plural** `mounts.*` (not `mount.*`). Each mount is identified by `mountId`.

**Important design notes:**
- `mounts.create` response does NOT include `serviceId`. Instead it has separate nullable fields for each service type: `applicationId`, `composeId`, `postgresId`, `mysqlId`, `mariadbId`, `mongoId`, `redisId`, `libsqlId`.
- Despite those per-service fields being null in responses, the `serviceId` field is correctly required on create and the backend routes it to the right service.
- The response also includes a `serviceType` field (`"application"`, `"compose"`, `"postgres"`, etc.) indicating which service type owns the mount.
- `mounts.update` accepts `mountId` + any writable fields and returns the full mount object.
- Delete verb is `mounts.remove` (`.delete` returns 404).
- `mounts.one` returns the full mount object plus embedded service sub-objects (all null except the owning service, which is also null if not populated by the query).

**Per-type required fields:**
- `bind`: `hostPath` is the correct field name. Server does NOT enforce it as required — a bind mount can be created with null `hostPath`. The Terraform provider should validate this at plan time.
- `volume`: `volumeName` is the correct field name. Server does NOT enforce it as required either.
- `file`: `content` is the correct field name (not `filePath` — `filePath` is a separate null field in the response). Server does NOT enforce `content` as required. The Terraform provider should validate at plan time.

---

### `GET /api/mounts.one?mountId=<id>`

Returns a single mount with embedded service sub-objects.

**Query params:** `mountId` (string, required).

**Response:** `200 application/json`

```json
{
  "mountId": "jn_e_BwMYE47qQnxOrv_l",
  "type": "bind",
  "hostPath": "/var/probe",
  "volumeName": null,
  "filePath": null,
  "content": null,
  "serviceType": "application",
  "mountPath": "/tmp/bind",
  "applicationId": null,
  "composeId": null,
  "libsqlId": null,
  "mariadbId": null,
  "mongoId": null,
  "mysqlId": null,
  "postgresId": null,
  "redisId": null,
  "application": null,
  "compose": null,
  "libsql": null,
  "mariadb": null,
  "mongo": null,
  "mysql": null,
  "postgres": null,
  "redis": null
}
```

**Note on `applicationId` being null:** Despite the mount being attached to an application, the individual service ID fields in the response can be null. Use `serviceType` + `mountId` as the canonical identifiers.

---

### `POST /api/mounts.create`

Creates a mount attached to a service.

**Request body:**
```json
{
  "type": "bind",
  "mountPath": "/tmp/bind",
  "serviceId": "88J5gWT57SBgtZ9ro4Y94",
  "hostPath": "/var/probe"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `serviceId` | string | yes | ID of the owning service (application, compose, postgres, etc.) |
| `type` | string | yes | `"bind"`, `"volume"`, or `"file"` |
| `mountPath` | string | yes | Path inside the container |
| `hostPath` | string | **required for bind** (server does not validate, validate at plan time) | Host filesystem path |
| `volumeName` | string | **required for volume** (server does not validate, validate at plan time) | Docker volume name |
| `content` | string | **required for file** (server does not validate, validate at plan time) | File content string |

**Response:** `200 application/json` — the created mount object (same shape as `mounts.one` without embedded service sub-objects).

---

### `POST /api/mounts.update`

Updates a mount.

**Request body:**
```json
{
  "mountId": "jn_e_BwMYE47qQnxOrv_l",
  "mountPath": "/tmp/bind-updated",
  "hostPath": "/var/probe-new"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `mountId` | string | yes | Mount ID |
| `mountPath` | string | no | New container path |
| `hostPath` | string | no | New host path (bind only) |
| `volumeName` | string | no | New volume name (volume only) |
| `content` | string | no | New file content (file only) |

**Response:** `200 application/json` — the full updated mount object (with embedded service sub-objects, all null).

---

### `POST /api/mounts.remove`

Deletes a mount. **Endpoint is `.remove`** (`.delete` returns 404).

**Request body:**
```json
{
  "mountId": "jn_e_BwMYE47qQnxOrv_l"
}
```

**Response:** `200 application/json` — the deleted mount object.

---

## port.*

> Verified against live instance on 2026-05-23.

Port mappings for applications. Each port is identified by `portId`. Note: the router uses singular `port.*` (not `ports.*`), but the `application.one` response embeds ports in a `ports[]` array.

**Key behaviors:**
- `port.create` and `port.update` both return the full port object.
- Delete verb is `port.delete` (`.remove` returns 404).
- `protocol` defaults to `"tcp"` if omitted on create.
- `publishMode` defaults to `"ingress"` and is always present in responses (not a user-settable field from Terraform).

---

### `GET /api/port.one?portId=<id>`

Returns a single port mapping with its full parent application embedded.

**Query params:** `portId` (string, required).

**Response:** `200 application/json`

```json
{
  "portId": "xaf-ZmbpqUVmFoWZmfaj2",
  "publishedPort": 8080,
  "publishMode": "ingress",
  "targetPort": 80,
  "protocol": "tcp",
  "applicationId": "88J5gWT57SBgtZ9ro4Y94",
  "application": { "...": "full application object" }
}
```

---

### `POST /api/port.create`

Creates a port mapping for an application.

**Request body:**
```json
{
  "applicationId": "88J5gWT57SBgtZ9ro4Y94",
  "publishedPort": 8080,
  "targetPort": 80,
  "protocol": "tcp"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `applicationId` | string | yes | Application ID |
| `publishedPort` | integer | yes | Host port to publish |
| `targetPort` | integer | yes | Container port to target |
| `protocol` | string | no | `"tcp"` (default) or `"udp"` |

**Response:** `200 application/json` — the created port object (without `application` sub-object):

```json
{
  "portId": "xaf-ZmbpqUVmFoWZmfaj2",
  "publishedPort": 8080,
  "publishMode": "ingress",
  "targetPort": 80,
  "protocol": "tcp",
  "applicationId": "88J5gWT57SBgtZ9ro4Y94"
}
```

---

### `POST /api/port.update`

Updates a port mapping.

**Request body:**
```json
{
  "portId": "xaf-ZmbpqUVmFoWZmfaj2",
  "publishedPort": 8080,
  "targetPort": 80,
  "protocol": "tcp"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `portId` | string | yes | Port ID |
| `publishedPort` | integer | no | New host port |
| `targetPort` | integer | no | New container port |
| `protocol` | string | no | `"tcp"` or `"udp"` |

**Response:** `200 application/json` — the full updated port object (same shape as create response).

---

### `POST /api/port.delete`

Deletes a port mapping. **Endpoint is `.delete`** (`.remove` returns 404).

**Request body:**
```json
{
  "portId": "xaf-ZmbpqUVmFoWZmfaj2"
}
```

**Response:** `200 application/json` — the deleted port object.

---

## notification.*

> Verified against live instance on 2026-05-23.

Notifications are configured at the organization level. There are 5 supported notification types: `slack`, `discord`, `email`, `telegram`, `gotify`. Each type has its own create and update endpoint. There is NO universal `notification.create` or `notification.update` — these endpoints return 404.

**Critical behaviors:**
- `notification.createSlack` (and all other type-specific creates) returns HTTP 200 with **empty body**. The `notificationId` must be obtained from `notification.all` after creation (match by `name` + `createdAt`).
- Each type-specific update (`notification.updateSlack`, `notification.updateDiscord`, etc.) requires both `notificationId` AND the type-specific sub-ID (`slackId`, `discordId`, etc.) from the `notification.one` response. Returns empty body on success.
- Delete verb is `notification.remove` (`.delete` returns 404). Returns the full deleted notification object.
- `notification.one` and `notification.all` return secrets in **plaintext** — `webhookUrl`, `botToken`, and email credentials are fully visible. The Terraform provider should treat these as sensitive and use state values for drift detection after the first read.

---

### `GET /api/notification.all`

Returns all notifications in the organization.

**Request:** no body, no query params.

**Response:** `200 application/json` — array of notification objects.

```json
[
  {
    "notificationId": "lkvThNFU1wuNjcqaIP7xr",
    "name": "my-slack",
    "appDeploy": true,
    "appBuildError": true,
    "databaseBackup": true,
    "volumeBackup": true,
    "dokployRestart": true,
    "dokployBackup": true,
    "dockerCleanup": true,
    "serverThreshold": true,
    "notificationType": "slack",
    "createdAt": "2026-05-23T04:52:52.999Z",
    "slackId": "dT9IdTYOobVyY7WN5Qrfg",
    "telegramId": null,
    "discordId": null,
    "emailId": null,
    "resendId": null,
    "gotifyId": null,
    "ntfyId": null,
    "mattermostId": null,
    "customId": null,
    "larkId": null,
    "pushoverId": null,
    "teamsId": null,
    "organizationId": "JYzDaUdW-hC0EX785HuXV",
    "slack": {
      "slackId": "dT9IdTYOobVyY7WN5Qrfg",
      "webhookUrl": "https://hooks.slack.com/services/T0/B0/X",
      "channel": "#test"
    },
    "telegram": null,
    "discord": null,
    "email": null,
    "resend": null,
    "gotify": null,
    "ntfy": null,
    "mattermost": null,
    "custom": null,
    "lark": null,
    "pushover": null,
    "teams": null
  }
]
```

---

### `GET /api/notification.one?notificationId=<id>`

Returns a single notification with full type-specific credentials.

**Query params:** `notificationId` (string, required).

**Response:** Same shape as a single element from `notification.all` (including the type sub-object with secrets in plaintext).

**Secret handling:** `webhookUrl` (Slack/Discord/Gotify), `botToken` (Telegram), and email credentials are returned in plaintext. The Terraform provider must store these in state and skip drift detection for secrets (use `state_value` where the API might not echo exactly what was set).

---

### `POST /api/notification.createSlack`

Creates a Slack notification.

**Request body:**
```json
{
  "name": "my-slack",
  "webhookUrl": "https://hooks.slack.com/services/T0/B0/X",
  "channel": "#alerts",
  "appDeploy": true,
  "appBuildError": true,
  "databaseBackup": true,
  "volumeBackup": true,
  "dokployRestart": true,
  "dokployBackup": true,
  "dockerCleanup": true,
  "serverThreshold": true
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Display name |
| `webhookUrl` | string | yes | Slack incoming webhook URL |
| `channel` | string | yes | Slack channel (e.g. `"#alerts"`) |
| `appDeploy` | boolean | yes | Notify on application deploy |
| `appBuildError` | boolean | yes | Notify on build error |
| `databaseBackup` | boolean | yes | Notify on database backup |
| `volumeBackup` | boolean | yes | Notify on volume backup |
| `dokployRestart` | boolean | yes | Notify on Dokploy restart |
| `dokployBackup` | boolean | yes | Notify on Dokploy backup |
| `dockerCleanup` | boolean | yes | Notify on Docker cleanup |
| `serverThreshold` | boolean | yes | Notify on server threshold alerts |

**Response:** HTTP 200 with **empty body**. Obtain the `notificationId` from `notification.all` after creation.

---

### `POST /api/notification.createDiscord`

Creates a Discord notification.

**Additional fields vs Slack:** `webhookUrl` (required), `decoration` (optional string).

**Response:** HTTP 200 with **empty body**.

---

### `POST /api/notification.createEmail`

Creates an Email notification.

**Additional fields vs Slack:** `smtpServer` (required), `smtpPort` (required integer), `username` (required), `password` (required), `fromAddress` (required), `toAddresses` (required, array of strings).

**Response:** HTTP 200 with **empty body**.

---

### `POST /api/notification.createTelegram`

Creates a Telegram notification.

**Additional fields vs Slack:** `botToken` (required), `chatId` (required), `messageThreadId` (optional).

**Response:** HTTP 200 with **empty body**.

---

### `POST /api/notification.createGotify`

Creates a Gotify notification.

**Additional fields vs Slack:** `serverUrl` (required), `appToken` (required), `priority` (optional integer), `decoration` (optional string).

**Response:** HTTP 200 with **empty body**.

---

### `POST /api/notification.updateSlack`

Updates a Slack notification. Requires both the `notificationId` AND `slackId` (the type-specific sub-ID from `notification.one`'s `slack.slackId` field).

**Request body:**
```json
{
  "notificationId": "lkvThNFU1wuNjcqaIP7xr",
  "slackId": "dT9IdTYOobVyY7WN5Qrfg",
  "name": "renamed-slack",
  "webhookUrl": "https://hooks.slack.com/services/T0/B0/Y",
  "channel": "#alerts",
  "appDeploy": true,
  "appBuildError": true,
  "databaseBackup": true,
  "volumeBackup": true,
  "dokployRestart": true,
  "dokployBackup": true,
  "dockerCleanup": true,
  "serverThreshold": true
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `notificationId` | string | yes | Notification ID |
| `slackId` | string | yes | Type-specific sub-ID from `notification.one`'s `slack.slackId` field |
| All create fields | — | yes | Same as `createSlack` |

**Response:** HTTP 200 with **empty body**.

---

### `POST /api/notification.updateDiscord`

Same pattern as `updateSlack` but requires `discordId` (from `notification.one`'s `discord.discordId`).

---

### `POST /api/notification.updateEmail`

Requires `emailId` (from `notification.one`'s `email.emailId`).

---

### `POST /api/notification.updateTelegram`

Requires `telegramId` (from `notification.one`'s `telegram.telegramId`).

---

### `POST /api/notification.updateGotify`

Requires `gotifyId` (from `notification.one`'s `gotify.gotifyId`).

---

### `POST /api/notification.remove`

Deletes a notification. **Endpoint is `.remove`** (`.delete` returns 404).

**Request body:**
```json
{
  "notificationId": "lkvThNFU1wuNjcqaIP7xr"
}
```

**Response:** `200 application/json` — the deleted notification object (without type sub-objects).

---

## application.* addendum: Swarm advanced fields

> Verified against live instance on 2026-05-23.

`application.update` accepts three additional fields for Docker Swarm mode deployments:

| Field | Type | Notes |
|-------|------|-------|
| `replicas` | integer | Number of service replicas; defaults to `1` |
| `healthCheckSwarm` | object\|null | Docker health check config (see shape below) |
| `restartPolicySwarm` | object\|null | Docker restart policy config (see shape below) |

Both swarm fields are `null` by default and are returned as `null` by `application.one` when not set.

### `healthCheckSwarm` shape

Fields use **PascalCase** matching the Docker Engine API. Durations are **nanosecond integers** (NOT Go-style "30s" strings).

```json
{
  "Test": ["CMD", "echo", "hi"],
  "Interval": 30000000000,
  "Timeout": 10000000000,
  "Retries": 3,
  "StartPeriod": 60000000000
}
```

| Field | Type | Notes |
|-------|------|-------|
| `Test` | string[] | Health check command array. `["CMD", ...]` for shell-escaped; `["CMD-SHELL", "..."]` for shell string |
| `Interval` | integer | Check interval in **nanoseconds** (30s = `30000000000`) |
| `Timeout` | integer | Check timeout in **nanoseconds** (10s = `10000000000`) |
| `Retries` | integer | Number of consecutive failures before marking unhealthy |
| `StartPeriod` | integer | Grace period in **nanoseconds** (60s = `60000000000`) |

### `restartPolicySwarm` shape

```json
{
  "Condition": "on-failure",
  "Delay": 5000000000,
  "MaxAttempts": 3,
  "Window": 120000000000
}
```

| Field | Type | Notes |
|-------|------|-------|
| `Condition` | string | One of `"none"`, `"on-failure"`, `"any"` |
| `Delay` | integer | Delay between restart attempts in **nanoseconds** (5s = `5000000000`) |
| `MaxAttempts` | integer | Maximum restart attempts before giving up |
| `Window` | integer | Evaluation window in **nanoseconds** (120s = `120000000000`) |

---

## backup.* addendum: compose and web-server types

> Verified against live instance on 2026-05-23.

### compose backups

`databaseType: "compose"` is **NOT supported** by the API. The Zod schema only accepts:
`"postgres"` | `"mariadb"` | `"mysql"` | `"mongo"` | `"web-server"` | `"libsql"`

Attempting `databaseType: "compose"` returns HTTP 400:
```json
{"fieldErrors": {"databaseType": ["Invalid option: expected one of \"postgres\"|\"mariadb\"|\"mysql\"|\"mongo\"|\"web-server\"|\"libsql\""]}}
```

**Conclusion:** Compose backups are not supported by the Dokploy backup API. The `backupType: "compose"` field visible on `backup.one` response objects is an internal field set for other purposes and cannot be created via `backup.create`. Do NOT add compose to the `database_type` enum on `dokploy_backup`.

### web-server (application) backups

`databaseType: "web-server"` IS accepted. The `database` field must be set to the `applicationId`. There is no typed ID field for web-server (unlike postgres/mysql/etc which require `postgresId`/`mysqlId`).

**Request body for web-server backup:**
```json
{
  "schedule": "0 3 * * *",
  "prefix": "app-backups/",
  "destinationId": "FwQFgPCZe4wKraiAd_dyd",
  "database": "<applicationId>",
  "databaseType": "web-server"
}
```

Optional: pass `applicationId` as an additional field (the server accepts it without error).

**Response:** HTTP 200 with **empty body** (same as other backup creates).

**V0.3 limitation — confirmed:** `application.one` does NOT include a `backups` key. Unlike postgres/mysql/mariadb/mongo which return `backups: []`, the application object has no backup-related keys whatsoever. There is no API endpoint to list web-server backups by application ID (`backup.listByApplicationId`, `backup.getByApplicationId`, etc. all return 404). The `backup.all` endpoint also does not exist (404).

**Implication for the Terraform provider:** The `dokploy_backup` resource for `database_type = "web-server"` cannot implement Read (cannot fetch existing backup by applicationId). The resource will always show as "created" after `terraform apply` but will be unable to detect drift or confirm the backup ID. This is a known API limitation — document in the resource schema and skip Read for web-server type, or treat it as write-only.

**Finding:** The existing v0.3 `listBackupsForResource` function in `backup.go` uses the parent resource's `.one` endpoint (e.g. `postgres.one`) to find backups. Since `application.one` has no `backups[]` field, the web-server backup type cannot be listed and the v0.3 limitation stands for v0.5 as well. No fix is possible without an API change.
