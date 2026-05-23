# Dokploy Terraform Provider — Databases (v0.2)

**Data:** 2026-05-22
**Versão alvo:** `lucasaarch/dokploy` v0.2.0
**Base:** v0.1.0 (já publicada no Terraform Registry)

## Objetivo

Adicionar suporte aos cinco principais bancos de dados gerenciados pelo
Dokploy, permitindo que uma stack `app + DB` seja provisionada num único
`terraform apply`.

## Escopo do v0.2

Cinco recursos novos:

```
dokploy_postgres    dokploy_mysql    dokploy_mariadb    dokploy_mongo    dokploy_redis
```

**Sem mudanças** nos recursos existentes (`dokploy_project`,
`dokploy_environment`, `dokploy_application`, `dokploy_domain`) nem no data
source `dokploy_organization`.

**Fora de escopo** (futuros): Compose services, Servers, Registries, Git
providers, Backups, Schedules, Notifications, libsql, configuração avançada
da Application (ports adicionais, mounts, replicas, health checks).

## Decisões de arquitetura

### Cinco recursos separados, helpers internos compartilhados

Cada banco tem schema próprio porque os atributos de autenticação variam
(Postgres tem `database_name`/`database_user`/`database_password`; MySQL e
MariaDB ainda têm `database_root_password`; Redis tem só
`database_password`). Cinco resources separados dão schemas tipados, validação
correta e autocomplete sensato — alternativa via discriminador `type` seria
mais compacta no código mas piora a UX.

Para evitar duplicação na orquestração create + deploy + polling (já
implementada uma vez para `dokploy_application`), um arquivo
`internal/provider/database_helpers.go` exporta:

- `deployAndWait(ctx, deploy func() error, getStatus func() (string, error), timeout time.Duration) error` —
  dispara o deploy, faz polling do status até `done`/`error`.
- `slugify(name string) string` — movido de `application_resource.go`.
- `generatePassword() string` — 32 chars de `[a-zA-Z0-9]` via `crypto/rand`,
  usado quando o usuário omite `database_password`/`database_root_password`.

Os 5 resources viram "casquinhas finas": schema + tradução `state ↔ client`.

### Camada de cliente

Cada banco recebe seu arquivo em `internal/client/`:

```
internal/client/
├── postgres.go     +   postgres_test.go
├── mysql.go        +   mysql_test.go
├── mariadb.go      +   mariadb_test.go
├── mongo.go        +   mongo_test.go
└── redis.go        +   redis_test.go
```

Cada um expõe o conjunto típico de métodos tipados:
`Create<DB>`, `Get<DB>`, `Update<DB>`, `Delete<DB>`, `Deploy<DB>`. Structs
`<DB>` (response) e `<DB>Input` (request). Idêntico em estilo a
`internal/client/application.go`.

## Configuração do provider

Nenhuma mudança — continua `endpoint` + `api_key` (via env vars
`DOKPLOY_ENDPOINT` / `DOKPLOY_API_KEY`).

## Recursos

### Atributos comuns aos cinco recursos

| Atributo | Tipo | Notas |
|---|---|---|
| `environment_id` | string, **obrigatório**, ForceNew | environment dono. |
| `name` | string, **obrigatório** | nome de exibição. |
| `description` | string, opcional | uma vez setado, removê-lo da config não limpa no servidor (mesma limitação documentada nos resources v0.1). |
| `docker_image` | string, **obrigatório** | versão da imagem (ex: `postgres:16`, `redis:7.2`). |
| `external_port` | number, opcional | quando setado, Dokploy publica a porta no host. |
| `env` | map(string), opcional | variáveis de ambiente extras. |
| `id` | string, **computed** | identificador do banco. |
| `app_name` | string, **computed** | nome interno gerado pelo Dokploy; usado como hostname na rede do Dokploy. |
| `status` | string, **computed** | status do último deploy (`idle`/`running`/`done`/`error`). |
| `timeouts` | block | `create`/`update`, default 10 min. |

### Atributos específicos por banco

#### `dokploy_postgres`
| Atributo | Tipo | Notas |
|---|---|---|
| `database_name` | string, **obrigatório** | nome do database inicial. |
| `database_user` | string, **obrigatório** | usuário do database. |
| `database_password` | string, opcional + **computed**, sensitive | omitido → provider gera. |

#### `dokploy_mysql` e `dokploy_mariadb`
| Atributo | Tipo | Notas |
|---|---|---|
| `database_name` | string, **obrigatório** | database inicial. |
| `database_user` | string, **obrigatório** | usuário do database. |
| `database_password` | string, opcional + **computed**, sensitive | omitido → provider gera. |
| `database_root_password` | string, opcional + **computed**, sensitive | omitido → provider gera. |

#### `dokploy_mongo`
| Atributo | Tipo | Notas |
|---|---|---|
| `database_user` | string, **obrigatório** | usuário root. |
| `database_password` | string, opcional + **computed**, sensitive | omitido → provider gera. |

#### `dokploy_redis`
| Atributo | Tipo | Notas |
|---|---|---|
| `database_password` | string, opcional + **computed**, sensitive | omitido → provider gera. |

## Comportamento das senhas

- `Optional + Computed`. Em Create: se o config omitir, o provider gera uma
  senha forte (32 chars `[a-zA-Z0-9]`, `crypto/rand`) e armazena no state. Se
  o config setar, usa o valor do config.
- Em Update: se o usuário remover o campo do config depois de tê-lo setado, o
  provider mantém o valor do state (não regenera, não envia vazio à API).
  Comportamento padrão de Optional+Computed.
- Mudança intencional do valor → update + re-deploy.
- **Aviso documentado no `MarkdownDescription`:** como os bancos usam volume
  persistente do Dokploy, mudar credenciais via Terraform afeta apenas
  containers recém-criados — em um banco já com dados, a credencial real
  dentro do DB não muda automaticamente (limitação das imagens oficiais, não
  do provider). Para rotacionar de fato a senha em um banco com dados, é
  preciso `terraform taint`/`replace` (recria o banco — perde dados) ou
  rotacionar via SQL/CLI dentro do banco.
- Sem geração de senha automática server-side, mesmo que a API do Dokploy
  ofereça: a geração client-side é mais simples (provider sempre sabe o
  valor sem round-trip extra) e consistente entre os 5 bancos.

## Fluxo de deploy automático

Idêntico ao `dokploy_application` da v0.1, agora abstraído em `deployAndWait`:

**Create:**
1. `<db>.create` → obtém `id`
2. `<db>.update` (ou endpoints `save*` equivalentes) → aplica `docker_image`,
   `external_port`, `env`, credenciais
3. `<db>.deploy` → dispara
4. Polling `<db>.one` lendo `applicationStatus` até `done` (sucesso) ou
   `error` (falha)

**Update:** aplica mudanças, redispara `<db>.deploy`, refaz polling.
Qualquer mudança em `docker_image` / `external_port` / `env` /
campos de auth dispara re-deploy.

**Falha de deploy:** se status virar `error`, `terraform apply` falha com
mensagem clara indicando o painel do Dokploy. O recurso **permanece no
state** com `status = "error"` — um próximo apply pode corrigir.

**Timeout:** bloco `timeouts` padrão (`create`/`update`, default 10 min).

## Drift, refresh e import

Mesmo padrão da v0.1:

- Read chama `<db>.one`. 404 → `IsNotFound(err)` → remove do state, recria no
  próximo plan.
- Valores alterados pelo painel aparecem como diff no plan.
- `terraform import dokploy_postgres.x <postgresId>` (e equivalentes) — Read
  preenche todos os atributos.
- **Drift de senhas:** se a API retornar a senha em `<db>.one`, drift normal.
  Se omitir (como `registry_password` faz em `application.one`), o provider
  não sobrescreve o state com o valor da API — drift externo nas senhas
  não é detectado. Verificado na Tarefa 1 do plano de implementação.

## Estratégia de testes

**Unitários** — TDD com `httptest` em `internal/client`, sem rede. Por banco:
- Path e método corretos para `Create`, `Get`, `Update`, `Delete`, `Deploy`.
- Header `x-api-key` enviado.
- Serialização do corpo (incluindo auth fields, `externalPort`).
- `IsNotFound` em 404.

**Aceitação (`TF_ACC=1`)** — contra `ship.sejablitz.com.br`, em `internal/provider`:

Por recurso (5 testes `TestAcc<DB>Resource`):
- create + Read (atributos computed setados)
- variante create-com-senha-omitida → confirma `database_password` setada com
  valor não-vazio de 32 chars
- update (muda `docker_image` ou `external_port`, confirma re-deploy)
- import (state bate)
- `CheckDestroy` (recurso removido do Dokploy)

Helper compartilhado novo em `internal/provider/helpers_test.go`:
`assertGeneratedPassword(t, resourceName, attr)` — confere 32 chars
`[a-zA-Z0-9]`.

**Sem alteração no `TestAccEndToEnd`.** Adicionar 5 bancos ao e2e atual o
tornaria muito lento (deploys reais somados). Cada banco já tem cobertura
ponta-a-ponta no seu próprio `TestAcc<DB>Resource`.

## Distribuição

Sem mudanças no pipeline (`.goreleaser.yml`, GitHub Actions).

Sequência de release:

1. Merge do v0.2 em `master`.
2. `git tag v0.2.0 && git push origin v0.2.0`.
3. GoReleaser publica GitHub Release assinado em ~4 min.
4. Terraform Registry detecta em ~5 min.
5. Versão `0.2.0` fica disponível em `lucasaarch/dokploy`.

`README.md` ganha as 5 novas linhas na seção "Resources". Docs auto-geradas
por `tfplugindocs` (já no `go generate`).

## Exemplo de uso (alvo)

```hcl
data "dokploy_organization" "current" {
  name = "Blitz IT Solutions"
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
  # database_password omitido → provider gera
}

resource "dokploy_redis" "cache" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-cache"
  docker_image   = "redis:7.2"
}

resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "registry.example.com/my-api:1.0"

  env = {
    DATABASE_URL = "postgres://app:${dokploy_postgres.db.database_password}@${dokploy_postgres.db.app_name}:5432/app"
    REDIS_URL    = "redis://:${dokploy_redis.cache.database_password}@${dokploy_redis.cache.app_name}:6379"
  }
}

output "db_password" {
  value     = dokploy_postgres.db.database_password
  sensitive = true
}
```

## Riscos e itens a verificar na implementação

1. **Nomes de endpoint dos 5 routers** — primeiro passo do plano: probe contra
   a instância real para confirmar `postgres.*`, `mysql.*`, `mariadb.*`,
   `mongo.*`, `redis.*` (e os métodos `create`/`one`/`update`/`deploy`/
   `remove`). Atualizar `internal/client/API.md` antes de codar.
2. **Formato dos payloads** — confirmar campos exatos (`databaseName` vs
   `database_name` vs `name`, `externalPort`, etc).
3. **Senhas em `<db>.one`** — confirmar se a API retorna ou omite. Define a
   lógica de drift (Read sobrescreve do API vs preserva state).
4. **Auto-geração server-side** — Dokploy provavelmente expõe; confirmar e
   documentar que escolhemos não usar (geração client-side é a estratégia).
5. **Campo de status** — confirmar que é `applicationStatus` mesmo (provável,
   dado o padrão), e quais valores aparecem para bancos (são containers, mas
   o ciclo pode ter peculiaridades — ex: bancos não rebuild, só "stopped" /
   "running").
