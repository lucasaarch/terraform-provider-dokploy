# Dokploy Terraform Provider — Backups & Schedules (v0.3)

**Data:** 2026-05-22
**Versão alvo:** `lucasaarch/dokploy` v0.3.0
**Base:** v0.2.0 (5 bancos + 4 resources do v0.1 + 1 data source — todas publicadas no Terraform Registry)

## Objetivo

Permitir provisionar via Terraform a infraestrutura de **backup** e **agendamento**
de uma stack Dokploy: configurar destinations S3, agendar backups recorrentes
de aplicações e bancos, e rodar comandos cron em containers de aplicação ou no
host Dokploy.

## Escopo do v0.3

Quatro novos recursos:

```
dokploy_destination               # S3-compat storage (org-level)
dokploy_backup                    # backups unificados (postgres/mysql/mariadb/mongo/web-server)
dokploy_application_schedule      # cron command no container da app
dokploy_host_schedule             # cron command no host do Dokploy
```

**Sem mudanças** nos recursos existentes (`dokploy_project`, `dokploy_environment`,
`dokploy_application`, `dokploy_domain`, `dokploy_postgres`, `dokploy_mysql`,
`dokploy_mariadb`, `dokploy_mongo`, `dokploy_redis`) nem no data source.

**Fora de escopo** (futuros): backups de `libsql` (sem resource ainda), schedules
do tipo `compose` (sem resource ainda), schedules do tipo `server` (sem resource
para servidores conectados ainda), restore de backup, listagem de backups
históricos.

## Decisões de arquitetura

### Backup unificado, schedules separados

A API do Dokploy é unificada para backup: `backup.create` aceita
`databaseType: "postgres"|"mariadb"|"mysql"|"mongo"|"web-server"|"libsql"`. O
schema do payload é idêntico nos 5 casos (`schedule`, `prefix`, `destinationId`,
`database`, `databaseType`). **Um único `dokploy_backup`** espelha esse design:
menos código, menos divergência, e mapeia 1:1 com a API.

Para schedules, a API também tem um único `schedule.create` com `scheduleType`,
mas o atributo de identidade muda: `applicationId` para `application`, nada para
`dokploy-server` (e `composeId`/`serverId` para tipos fora de escopo). **Dois
resources separados** evitam validação condicional no schema (`if type=X então
campo Y obrigatório`) e dão semântica clara por arquivo. O cliente HTTP é único
(`internal/client/schedule.go`) — apenas os payloads enviados por cada resource
diferem.

### Nomenclatura `host_schedule` (e não `server_schedule`)

A API tem dois `scheduleType`s: `server` (servidor gerenciado conectado) e
`dokploy-server` (a própria host). Para evitar confusão futura quando
adicionarmos servers gerenciados, o resource v0.3 chama-se `dokploy_host_schedule`
— claro que é o host do Dokploy, livre pro `dokploy_server_schedule` aparecer
depois.

### Sem deploy step

Backups e schedules são puras configurações: a API não tem `backup.deploy` ou
`schedule.deploy`. Resources viram CRUD simples — não precisam de `deployAndWait`
nem de bloco `timeouts`.

### Camada de cliente

```
internal/client/
├── destination.go     +   destination_test.go
├── backup.go          +   backup_test.go
└── schedule.go        +   schedule_test.go   # compartilhado pelos 2 resources de schedule
```

Cada arquivo expõe métodos `Create<X>`, `Get<X>`, `Update<X>`, `Delete<X>` e
structs `<X>` (response) e `<X>Input` (request). Mesmo estilo de
`internal/client/application.go` e `internal/client/postgres.go`.

## Configuração do provider

Nenhuma mudança — continua `endpoint` + `api_key`.

## Recursos

### `dokploy_destination`

Configuração de armazenamento S3-compatível ao nível da organização. Usado por
um ou mais `dokploy_backup`.

| Atributo | Tipo | Notas |
|---|---|---|
| `name` | string, **obrigatório** | nome do destination. |
| `provider` | string, **obrigatório**, ForceNew | enum de S3 provider; valores exatos confirmados na Tarefa 1 do plano (provável `aws`/`digital_ocean`/`cloudflare`/`custom` com a capitalização que a API usa). |
| `bucket` | string, **obrigatório** | nome do bucket. |
| `endpoint` | string, **obrigatório** | URL S3 (ex: `https://sfo3.digitaloceanspaces.com`, `https://<account>.r2.cloudflarestorage.com`). |
| `region` | string, opcional | obrigatório pra AWS; vazio em DO Spaces / R2. |
| `access_key` | string, **obrigatório** | chave de acesso. Não marcado sensitive (segue convenção AWS). |
| `secret_access_key` | string, **obrigatório**, sensitive | mascarado em outputs/CLI. |
| `additional_flags` | list(string), opcional | flags extras passadas pra ferramenta de backup. |
| `id` | string, **computed** | `destinationId`. |
| `organization_id` | string, **computed** | organização (vem da API key). |

**Drift de credenciais:** `destination.all` retorna `secretAccessKey` em
plaintext na API. Read sobrescreve do API normalmente — drift externo nas 5
fields editáveis é detectado.

### `dokploy_backup`

Configuração de backup automático. Um destination + um cron + um alvo.

| Atributo | Tipo | Notas |
|---|---|---|
| `database_type` | string, **obrigatório**, ForceNew | enum: `postgres` / `mysql` / `mariadb` / `mongo` / `web-server`. |
| `database_id` | string, **obrigatório**, ForceNew | FK ao recurso correspondente (`dokploy_postgres.x.id`, `dokploy_application.x.id`, etc). |
| `destination_id` | string, **obrigatório** | `dokploy_destination.x.id`. |
| `schedule` | string, **obrigatório** | cron (ex: `0 3 * * *`). |
| `prefix` | string, **obrigatório** | path prefix dentro do bucket. |
| `enabled` | bool, opcional + computed | só se a API expuser; default `true`. **Confirmar na Tarefa 1.** |
| `keep_latest_count` | number, opcional + computed | retenção (manter N últimos). **Confirmar na Tarefa 1** se a API tem esse campo. Se não tiver, removo do schema. |
| `id` | string, **computed** | `backupId`. |

**Validação:** o provider valida que `database_type` é um dos 5 valores conhecidos
no plan-time (via `stringvalidator.OneOf`). Coerência `database_type` ↔ tipo
real do `database_id` só falha no apply (erro do servidor propagado).

### `dokploy_application_schedule`

Cron de comando dentro do container de uma application.

| Atributo | Tipo | Notas |
|---|---|---|
| `application_id` | string, **obrigatório**, ForceNew | `dokploy_application.x.id`. Cliente envia `scheduleType: "application"`. |
| `name` | string, **obrigatório** | nome do schedule. |
| `cron_expression` | string, **obrigatório** | ex: `0 3 * * *`. |
| `command` | string, **obrigatório** | comando shell. |
| `shell_type` | string, opcional + computed | default `bash`. API aceita `bash`/`sh`. |
| `enabled` | bool, opcional + computed | default `true`. Atualizável (pausa sem destruir). |
| `timezone` | string, opcional | ex: `America/Sao_Paulo`. Null → UTC. |
| `id` | string, **computed** | `scheduleId`. |
| `app_name` | string, **computed** | nome interno (`schedule-...`). |

### `dokploy_host_schedule`

Cron rodando no host do Dokploy.

| Atributo | Tipo | Notas |
|---|---|---|
| `name` | string, **obrigatório** | |
| `cron_expression` | string, **obrigatório** | |
| `command` | string, **obrigatório** | |
| `shell_type` | string, opcional + computed | default `bash`. |
| `enabled` | bool, opcional + computed | default `true`. |
| `timezone` | string, opcional | |
| `id` | string, **computed** | `scheduleId`. |
| `app_name` | string, **computed** | |

Cliente envia `scheduleType: "dokploy-server"` e sem `applicationId`.

## Drift, refresh e import

Mesmo padrão dos resources existentes:

- Read chama `<router>.one`. `404` → `IsNotFound(err)` → remove do state.
- Out-of-band changes via painel aparecem como diff no plan.
- `terraform import dokploy_destination.x <destinationId>` (e equivalentes) — Read
  preenche todos os atributos.
- `terraform import dokploy_backup.x <backupId>` — Read traz `database_type`,
  `database_id`, etc. do retorno do `backup.one`.

## Estratégia de testes

**Unitários** — TDD com `httptest`, sem rede. Por router (`destination`, `backup`,
`schedule`):
- Path e método de `Create`, `Get`, `Update`, `Delete`.
- Header `x-api-key`.
- Serialização do corpo (incluindo campos sensíveis em destination, e o
  discriminator `databaseType` em backup, e o `scheduleType` em schedule).
- `IsNotFound` em 404.

**Aceitação (`TF_ACC=1`)** contra `ship.sejablitz.com.br`. Cinco testes
de aceitação:

- **`TestAccDestinationResource`** — create + update do `name` + import + destroy.
  Verifica que `secret_access_key` é redacted em diags do plan mas presente no state.
- **`TestAccBackupResource`** — cria postgres temporário + destination temporário
  + backup. Update do `schedule`. Import. Destroy.
- **`TestAccBackup_WebServer`** — cria application temporária + destination + backup
  `database_type = "web-server"`. Cobre o tipo `web-server` sem duplicar a estrutura
  dos demais (postgres já cobre o caminho do código).
- **`TestAccApplicationScheduleResource`** — cria application + schedule, update do
  `command`, import, destroy.
- **`TestAccHostScheduleResource`** — cria schedule no host (sem dependência),
  update, import, destroy.

Total: 5 testes acceptance novos. **Sem mudança no `TestAccEndToEnd`** — adicionar
4 resources lentos ao e2e tornaria o teste massivo.

## Distribuição

Sem mudanças no pipeline (`.goreleaser.yml`, GitHub Actions).

Após merge do v0.3 em `master`:

```bash
git tag v0.3.0 && git push origin v0.3.0
```

`README.md` ganha 4 linhas novas em "Resources". Docs auto-geradas.

## Exemplo de uso (alvo)

```hcl
data "dokploy_organization" "current" {
  name = "Blitz IT Solutions"
}

resource "dokploy_destination" "s3" {
  name     = "prod-backups"
  provider = "digital_ocean"
  bucket   = "blitz-backups"
  endpoint = "https://sfo3.digitaloceanspaces.com"

  access_key        = var.do_access_key
  secret_access_key = var.do_secret_key
}

resource "dokploy_project" "app" {
  name = "minha-app"
}

resource "dokploy_postgres" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
}

resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "registry.example.com/my-api:1.0"
}

# Backup diário do banco
resource "dokploy_backup" "db_daily" {
  database_type  = "postgres"
  database_id    = dokploy_postgres.db.id
  destination_id = dokploy_destination.s3.id
  schedule       = "0 3 * * *"
  prefix         = "postgres/app/"
}

# Backup semanal dos volumes da app
resource "dokploy_backup" "app_weekly" {
  database_type  = "web-server"
  database_id    = dokploy_application.api.id
  destination_id = dokploy_destination.s3.id
  schedule       = "0 4 * * 0"
  prefix         = "web-server/api/"
}

# Cron warmup dentro do container da app
resource "dokploy_application_schedule" "warmup" {
  application_id  = dokploy_application.api.id
  name            = "warmup-cache"
  cron_expression = "*/15 * * * *"
  command         = "curl -s http://localhost:3000/internal/warmup"
}

# Cron de limpeza no host do Dokploy
resource "dokploy_host_schedule" "rotate_logs" {
  name            = "rotate-traefik-logs"
  cron_expression = "0 0 * * *"
  command         = "find /var/log/dokploy -name '*.log.*' -mtime +14 -delete"
  timezone        = "America/Sao_Paulo"
}
```

## Riscos e itens a verificar na implementação

1. **Enum exato de `destination.provider`** — visto `"DigitalOcean"`. Probar
   todos os valores aceitos via Zod error de `destination.create` com `provider`
   inválido (a API retorna a lista no erro). Confirmar capitalização.
2. **Campos opcionais de `backup.create`** — só os 5 required foram observados.
   Probar com payload completo e ver o response (procurar `enabled`,
   `keepLatestCount`, `serviceName`, etc). Ajustar o schema do resource
   conforme o que existir.
3. **Response shape de `backup.one`** — confirmar que retorna `databaseType` e
   `database` (o id) pra que o Read consiga popular o state com fidelidade.
4. **`schedule.update` shape** — visto que `delete` é `schedule.delete` (não
   `.remove`). Confirmar que `update` é `schedule.update` ou outro nome.
5. **Senhas em `destination.one`** — vista que `destination.all` retorna
   `secretAccessKey` em plaintext. Confirmar mesmo no `destination.one`. Se
   omitir, usar padrão "preservar state" (igual `registry_password` da v0.1).
6. **`enabled`/`scheduleType`** — confirmar que update do `enabled`
   funciona sem precisar re-criar; que mudar `scheduleType` está bloqueado
   (faz sentido como ForceNew implícito — application_schedule e host_schedule
   são resources diferentes, então o usuário não consegue trocar via
   Terraform).
