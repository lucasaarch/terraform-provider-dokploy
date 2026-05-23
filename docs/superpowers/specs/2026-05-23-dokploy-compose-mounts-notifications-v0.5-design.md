# Dokploy Terraform Provider — Compose, Mounts, Ports, Notifications & Advanced App Config (v0.5)

**Data:** 2026-05-23
**Versão alvo:** `lucasaarch/dokploy` v0.5.0
**Base:** v0.4.0 (já publicada no Terraform Registry)

## Objetivo

Adicionar suporte a **stacks Docker Compose**, **mounts** (volumes/binds/files
em qualquer service Dokploy), **portas publicadas** em applications,
**configuração avançada** de applications (replicas, healthcheck, restart
policy), e **notifications** (Slack/Discord/Email/Telegram/Gotify).

## Escopo do v0.5

**8 novos resources:**

```
dokploy_compose                       # stack docker-compose
dokploy_mount                         # bind/volume/file mount em qualquer service
dokploy_port                          # porta publicada em application
dokploy_slack_notification            # notificação Slack
dokploy_discord_notification          # notificação Discord
dokploy_email_notification            # notificação Email (SMTP)
dokploy_telegram_notification         # notificação Telegram
dokploy_gotify_notification           # notificação Gotify
```

**1 resource existente modificado (additive):**

```
dokploy_application                   # ganha replicas, health_check, restart_policy
dokploy_backup                        # ganha "compose" no enum database_type
```

**Fora de escopo** (futuros): Compose com source Git (só `raw` no v0.5),
configuração Swarm completa (placement, networks, labels, ulimits — só os
3 essenciais: replicas/healthcheck/restart_policy), webhook genérico
(Dokploy não tem `notification.createWebhook` — não existe na API).

## Decisões de arquitetura

### Compose segue o padrão da Application

`dokploy_compose` tem o mesmo lifecycle de `dokploy_application`: create →
configure → deploy → poll status. Reusa `deployAndWait` (do
`database_helpers.go`) e o helper `slugify`. Cliente em
`internal/client/compose.go`. Pra v0.5, source é só `"raw"` (string YAML
inline); fontes Git ficam pra v0.9.

### Mount unificado com discriminator

A API tem **um único** `mounts.create` que aceita `type: "bind" | "volume" | "file"`
com campos condicionais. Modelamos como **um único `dokploy_mount`** com
`type` + `OneOf` validator + campos condicionais. Mesma escolha que fizemos
em `dokploy_backup` no v0.3 (API unificada → resource unificado).

`service_id` é um FK polimórfico — pode apontar para uma application,
postgres, mysql, mariadb, mongo, redis, ou compose. Terraform não consegue
validar o tipo no plan; quem rejeita FK inválido é a API.

### Port é simples e standalone

`dokploy_port` é o equivalente de `dokploy_domain` pra mapeamentos de porta
brutos (host:published → container:target). Cada port é um resource separado
(múltiplas portas = múltiplos resources). Spec deliberadamente simples — sem
`protocol` por agora (default tcp).

### Notifications: 5 resources separados

A API expõe endpoints type-específicos (`notification.createSlack`,
`notification.createDiscord`, etc) com schemas muito diferentes (Email tem
SMTP, Telegram tem botToken, Slack tem webhookUrl+channel). Modelamos como
**5 resources separados** — mesma escolha que os 5 DBs no v0.2.

Cliente HTTP compartilhado em `internal/client/notification.go`: 5
`Create<Type>` methods + 1 `GetNotification` + 1 `UpdateNotification` (por
type também?) + 1 `DeleteNotification` universal. A Tarefa 1 do plano
confirma se update é universal ou tipo-específico.

**Campos de eventos comuns** (presentes em todos os 5):
`appDeploy`, `appBuildError`, `databaseBackup`, `dokployBackup`,
`volumeBackup`, `dokployRestart`, `dockerCleanup`, `serverThreshold`.

**Campos específicos** (por tipo):
- Slack: `webhookUrl`, `channel`
- Discord: `webhookUrl`, `decoration`
- Email: `smtpServer`, `smtpPort`, `username`, `password`, `fromAddress`, `toAddresses`
- Telegram: `botToken`, `chatId`, `messageThreadId`
- Gotify: `serverUrl`, `appToken`, `priority`, `decoration`

### Application advanced via nested blocks

`replicas` é um simple Int. `health_check` e `restart_policy` viram **nested
single blocks** (`schema.SingleNestedBlock`) — mapeiam pros objetos JSON
`healthCheckSwarm` e `restartPolicySwarm` da API. Mais natural em HCL do que
flat attributes:

```hcl
resource "dokploy_application" "api" {
  # ... existing fields ...
  replicas = 3

  health_check {
    test          = ["CMD", "curl", "-f", "http://localhost:3000/health"]
    interval      = "30s"
    timeout       = "10s"
    retries       = 3
    start_period  = "60s"
  }

  restart_policy {
    condition     = "on-failure"
    delay         = "5s"
    max_attempts  = 3
    window        = "120s"
  }
}
```

Mudanças nesses 3 campos disparam re-deploy da application (igual mudança em
docker_image).

### Backup ganha "compose" no enum

`dokploy_backup.database_type` ganha o valor `"compose"` no `OneOf` validator.
Cliente do backup já aceita o campo `composeId` no `Backup` struct (v0.3), só
precisa adicionar suporte em `SetTypedID` e em `listBackupsForResource`.
Resolve o sibling do "web-server backup" pra compose.

### Camada de cliente

```
internal/client/
├── compose.go        + compose_test.go        # NOVO
├── mount.go          + mount_test.go          # NOVO
├── port.go           + port_test.go           # NOVO
├── notification.go   + notification_test.go   # NOVO
├── application.go                              # MOD — adicionar healthCheckSwarm/restartPolicySwarm
└── backup.go                                   # MOD — SetTypedID lida com "compose"
```

## Configuração do provider

Sem mudanças.

## Recursos

### `dokploy_compose`

Stack docker-compose. Lifecycle igual application.

| Atributo | Tipo | Notas |
|---|---|---|
| `environment_id` | string, **obrigatório**, ForceNew | environment dono. |
| `name` | string, **obrigatório** | |
| `description` | string, opcional | mesma limitação dos outros (uma vez setado, não dá pra limpar via API). |
| `compose_file` | string, **obrigatório** | conteúdo YAML do docker-compose.yml. |
| `source_type` | string, opcional + computed | enum, v0.5 só aceita `"raw"`. Default `"raw"`. |
| `env` | map(string), opcional | env vars passadas pro compose. |
| `server_id` | string, opcional + computed, ForceNew | `dokploy_server.x.id`. Omitir = host do Dokploy. |
| `id` | string, **computed** | `composeId`. |
| `app_name` | string, **computed** | nome interno gerado pelo Dokploy. |
| `status` | string, **computed** | status do último deploy. |
| `timeouts` | block | `create`/`update`, default 10 min. |

Lifecycle Create:
1. `compose.create` → obtém `composeId`
2. `compose.update` (ou endpoints `save*` equivalentes — confirmar Tarefa 1) → aplica `compose_file`, `env`, `source_type`
3. `compose.deploy` → dispara
4. Polling até `done`/`error`

Update: redispara deploy.

### `dokploy_mount`

Sub-resource attached to any service via `service_id`. Type-discriminated.

| Atributo | Tipo | Notas |
|---|---|---|
| `service_id` | string, **obrigatório**, ForceNew | id de application/compose/postgres/mysql/etc. Trocar = mount diferente. |
| `type` | string, **obrigatório**, ForceNew | enum `OneOf("bind", "volume", "file")`. |
| `mount_path` | string, **obrigatório** | caminho dentro do container. |
| `host_path` | string, opcional | apenas para `type = "bind"`. Caminho no host. |
| `volume_name` | string, opcional | apenas para `type = "volume"`. Nome do volume Docker. |
| `content` | string, opcional, sensitive | apenas para `type = "file"`. Conteúdo do arquivo a ser injetado. |
| `id` | string, **computed** | `mountId`. |

**Validação cruzada (no plan-time):**
- `type = "bind"` → `host_path` obrigatório; `volume_name` e `content` devem ser null.
- `type = "volume"` → `volume_name` obrigatório; `host_path` e `content` devem ser null.
- `type = "file"` → `content` obrigatório; `host_path` e `volume_name` devem ser null.

Implementação: `ConfigValidators` no resource cuida disso (`validatorhelper.RequiredIfStringEquals` ou validators custom).

### `dokploy_port`

Mapeamento de porta publicada.

| Atributo | Tipo | Notas |
|---|---|---|
| `application_id` | string, **obrigatório**, ForceNew | aplicação que recebe a porta. |
| `published_port` | number, **obrigatório** | porta no host. |
| `target_port` | number, **obrigatório** | porta no container. |
| `protocol` | string, opcional + computed | default `"tcp"`. (Aceita `"tcp"` ou `"udp"` — confirmar na Tarefa 1.) |
| `id` | string, **computed** | `portId`. |

### Notifications — 5 resources

**Atributos de evento comuns aos 5 resources** (todos `bool`, opcional+computed, default depende da API):

| Atributo | Default sugerido |
|---|---|
| `app_deploy` | `true` |
| `app_build_error` | `true` |
| `database_backup` | `true` |
| `dokploy_backup` | `true` |
| `volume_backup` | `true` |
| `dokploy_restart` | `true` |
| `docker_cleanup` | `true` |
| `server_threshold` | `true` |
| `name` | sem default — required |
| `id` | computed |

Quem decide os defaults: a Tarefa 1 do plano confirma. Se a API exige todos, marcamos Required; se aceita ausência, deixamos Optional+Computed.

**Específico por tipo:**

#### `dokploy_slack_notification`
| Atributo | Tipo | Notas |
|---|---|---|
| `webhook_url` | string, **obrigatório**, sensitive | URL do webhook Slack. |
| `channel` | string, **obrigatório** | nome do canal (#deploys). |

#### `dokploy_discord_notification`
| Atributo | Tipo | Notas |
|---|---|---|
| `webhook_url` | string, **obrigatório**, sensitive | URL do webhook Discord. |
| `decoration` | bool, opcional + computed | enable emoji/embeds. |

#### `dokploy_email_notification`
| Atributo | Tipo | Notas |
|---|---|---|
| `smtp_server` | string, **obrigatório** | host SMTP. |
| `smtp_port` | number, **obrigatório** | porta SMTP. |
| `username` | string, **obrigatório** | user SMTP. |
| `password` | string, **obrigatório**, sensitive | senha SMTP. |
| `from_address` | string, **obrigatório** | from. |
| `to_addresses` | list(string), **obrigatório** | destinatários. |

#### `dokploy_telegram_notification`
| Atributo | Tipo | Notas |
|---|---|---|
| `bot_token` | string, **obrigatório**, sensitive | token do bot. |
| `chat_id` | string, **obrigatório** | id do chat/grupo. |
| `message_thread_id` | string, opcional | id de tópico (forum group). |

#### `dokploy_gotify_notification`
| Atributo | Tipo | Notas |
|---|---|---|
| `server_url` | string, **obrigatório** | URL do servidor Gotify. |
| `app_token` | string, **obrigatório**, sensitive | token da app no Gotify. |
| `priority` | number, opcional + computed | prioridade da notificação (1-10). |
| `decoration` | bool, opcional + computed | enable emoji/decoration. |

**Update e Delete**: a Tarefa 1 confirma se há `notification.update` universal ou tipo-específico, e se `notification.remove` ou `.delete`.

### `dokploy_application` — campos novos

Mantém todos os atributos atuais. Adiciona:

| Atributo | Tipo | Notas |
|---|---|---|
| `replicas` | number, opcional + computed | número de réplicas (Docker Swarm). |
| `health_check` | nested single block, opcional | objeto com `test` (list of strings), `interval` (string Go duration "30s"), `timeout` (string), `retries` (number), `start_period` (string). |
| `restart_policy` | nested single block, opcional | objeto com `condition` (string: "none" / "on-failure" / "any"), `delay` (string), `max_attempts` (number), `window` (string). |

Mudança em qualquer um dispara update + re-deploy (mesma lógica do `docker_image`).

## Drift, refresh e import

Padrão idêntico ao já estabelecido:
- Read chama `<router>.one`. 404 → IsNotFound → remove do state.
- Import por `<routerId>` (cada resource pelo seu id).
- Notificações: `private_key`/senhas SMTP/etc são retornados em plaintext pela API? Confirmar na Tarefa 1; se não, preservar state.

## Estratégia de testes

**Unitários** — TDD com `httptest`:
- `internal/client/compose_test.go`
- `internal/client/mount_test.go` — testa as 3 variantes (bind/volume/file)
- `internal/client/port_test.go`
- `internal/client/notification_test.go` — testa os 5 create methods
- Mudanças em `application.go` cobertas pelo teste existente + 1 caso novo com `replicas`

**Aceitação (`TF_ACC=1`)** contra `ship.sejablitz.com.br`:

| Teste | Notas |
|---|---|
| `TestAccComposeResource` | Stack compose pequena (ex: `redis:7-alpine`), deploy real, update, import, destroy. |
| `TestAccMountResource_Bind` | Mount tipo `bind` em uma application temporária. |
| `TestAccMountResource_Volume` | Mount tipo `volume`. |
| `TestAccMountResource_File` | Mount tipo `file` (config inline). |
| `TestAccPortResource` | Port em application; testa update do target_port. |
| `TestAccSlackNotificationResource` | Slack com webhook fake (`https://hooks.slack.com/services/T0/B0/X` não-funcional mas Zod aceita). |
| `TestAccDiscordNotificationResource` | Discord idem. |
| `TestAccEmailNotificationResource` | Email com SMTP fake (`smtp.example.com:587`). |
| `TestAccTelegramNotificationResource` | Telegram com bot fake. |
| `TestAccGotifyNotificationResource` | Gotify com server fake. |
| `TestAccApplicationResource_Advanced` | application com replicas + health_check + restart_policy; verifica que survives import. |

11 testes novos. Notificações não enviam mensagem real (URLs/tokens fake) — só testam o CRUD do registro no Dokploy.

## Distribuição

Sem mudança no pipeline. Após merge:

```bash
git tag v0.5.0 && git push origin v0.5.0
```

`README.md` ganha 8 linhas novas em "Resources".

## Exemplo de uso (alvo)

```hcl
data "dokploy_organization" "current" {
  name = "Blitz IT Solutions"
}

resource "dokploy_project" "obs" {
  name = "observability"
}

# Notificação que dispara em todos os eventos
resource "dokploy_slack_notification" "alerts" {
  name        = "alerts"
  webhook_url = var.slack_webhook
  channel     = "#deploys"
}

# Stack docker-compose com Prometheus + Grafana
resource "dokploy_compose" "monitoring" {
  environment_id = dokploy_project.obs.production_environment_id
  name           = "monitoring"

  compose_file = file("${path.module}/monitoring.yml")
  env = {
    GRAFANA_ADMIN_PASSWORD = var.grafana_password
  }
}

# Volume persistente pro Prometheus dentro da stack
resource "dokploy_mount" "prometheus_data" {
  service_id   = dokploy_compose.monitoring.id
  type         = "volume"
  mount_path   = "/prometheus"
  volume_name  = "prometheus-data"
}

# Config Prometheus injetado como arquivo
resource "dokploy_mount" "prometheus_config" {
  service_id = dokploy_compose.monitoring.id
  type       = "file"
  mount_path = "/etc/prometheus/prometheus.yml"
  content    = file("${path.module}/prometheus.yml")
}

# Aplicação web com health check e 3 réplicas
resource "dokploy_application" "api" {
  environment_id = dokploy_project.obs.production_environment_id
  name           = "api"
  docker_image   = "registry.example.com/my-api:1.0"

  replicas = 3

  health_check {
    test         = ["CMD", "curl", "-f", "http://localhost:3000/health"]
    interval     = "30s"
    timeout      = "10s"
    retries      = 3
    start_period = "60s"
  }

  restart_policy {
    condition    = "on-failure"
    delay        = "5s"
    max_attempts = 3
    window       = "120s"
  }
}

resource "dokploy_port" "api_metrics" {
  application_id = dokploy_application.api.id
  published_port = 9090
  target_port    = 9090
}
```

## Riscos e itens a verificar na implementação

1. **`compose.update` ou save endpoints**: confirmar como o YAML / env são persistidos (provavelmente `compose.update` + `compose.deploy`).
2. **`compose.create` retorna body cheio ou vazio?** Se vazio (como sshKey/backup), usar discovery via parent.
3. **Defaults de eventos das notifications**: a API aceita ausência ou exige todos os 8 bools? Define se schema é Required ou Optional+Computed.
4. **`notification.update` é universal ou tipo-específico?** (`updateSlack`/`updateDiscord`/etc?). Define a estrutura do cliente.
5. **Webhook URLs e tokens retornados em plaintext em `notification.one`?** Define drift detection.
6. **`mounts.update` aceita mudar `type`?** Provavelmente não — confirmar e marcar ForceNew se sim.
7. **`port.create` aceita `protocol`?** Se sim, default `tcp`. Se não, omitir do schema.
8. **`application.update` aceita `replicas`/`healthCheckSwarm`/`restartPolicySwarm`?** Confirmar shapes (objetos JSON aninhados).
9. **`backup.create` com `databaseType = "compose"` e `composeId`?** Verificar shape. Se funciona, estender o enum do `dokploy_backup` resource.
10. **Web-server backup (v0.3 limit)**: investigar se aplicações com mounts configurados expõem `backups[]` ou se há outro endpoint (`volumeBackup.*`?).
