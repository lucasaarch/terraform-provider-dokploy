# Dokploy Terraform Provider — Servers & SSH Keys (v0.4)

**Data:** 2026-05-23
**Versão alvo:** `lucasaarch/dokploy` v0.4.0
**Base:** v0.3.0 (já publicada no Terraform Registry)

## Objetivo

Permitir provisionar via Terraform a infraestrutura de **servidores remotos**
no Dokploy: registrar chaves SSH, conectar máquinas remotas como workers
gerenciados, e direcionar applications, databases e schedules pra rodar nesses
servidores em vez do host do Dokploy.

## Escopo do v0.4

**3 novos resources:**

```
dokploy_ssh_key             # chave SSH armazenada no Dokploy (organização)
dokploy_server              # servidor remoto registrado via sshKey
dokploy_server_schedule     # cron rodando em um server gerenciado
```

**6 resources existentes ganham atributo `server_id` (Optional, ForceNew):**

```
dokploy_application
dokploy_postgres
dokploy_mysql
dokploy_mariadb
dokploy_mongo
dokploy_redis
```

**Fora de escopo** (futuros): `scheduleType: "compose"` (sem `dokploy_compose`),
`buildServerId` em application (sem suporte a fontes Git ainda — v0.4 segue
suportando só Docker), gerenciamento da chave pública no `authorized_keys` da
VM remota (responsabilidade do usuário, via cloud-init/ansible/null_resource).

## Decisões de arquitetura

### `dokploy_server` exige handshake SSH real

A API `server.create` faz o handshake SSH no momento da criação. Se a chave
pública do `dokploy_ssh_key` ainda não estiver em `authorized_keys` da VM
remota, `terraform apply` falha. O fluxo correto é:

1. `dokploy_ssh_key` (provider gera ou usuário fornece chaves)
2. Usuário cola `public_key` em `~/.ssh/authorized_keys` da VM (manual, ou
   via cloud-init/null_resource — fora do escopo do provider)
3. `dokploy_server` (faz o handshake)

Quando o handshake falha, o `dokploy_ssh_key` permanece (foi criado), só o
`dokploy_server` que não. Próximo apply tenta de novo após o usuário consertar
o authorized_keys.

### `server_id` é ForceNew em todos os 6 resources

A API aceita update inline, mas mover **um banco já existente** pra outro server
implicaria perda de dados (o volume Docker fica na máquina antiga). Pra evitar
perda silenciosa, todos os 6 resources marcam `server_id` como
`RequiresReplace()` — mudar dispara destroy+create explícito, deixando claro o
custo. Inclui application (Docker, stateless) por consistência e simplicidade.

### Geração client-side de chaves SSH

Quando o usuário omite `private_key`/`public_key` no `dokploy_ssh_key`, o
provider gera um par **RSA 4096** via `crypto/rsa` e serializa:

- Privada: PEM PKCS#1 (`-----BEGIN RSA PRIVATE KEY-----`)
- Pública: formato OpenSSH (`ssh-rsa AAAA... <name>`) via
  `ssh.MarshalAuthorizedKey`

Ambas ficam no state Terraform (privada marcada `Sensitive`). Mesma decisão
que as senhas dos databases no v0.2.

### Dependência nova: `golang.org/x/crypto`

Adicionada ao `go.mod` como dependência direta (já é transitiva via
`terraform-plugin-framework`). Usada apenas em
`internal/provider/database_helpers.go::generateSSHKeyPair`.

### Compatibilidade com v0.3

Todas as mudanças nos resources existentes são **additive**: o atributo
`server_id` é Optional+Computed. Usuários do v0.3 atualizando pro v0.4 sem
mexer no HCL não vêem diff. Quem adicionar `server_id` num resource já
existente vai ver replacement — o comportamento correto.

### Camada de cliente

```
internal/client/
├── sshkey.go     +   sshkey_test.go     # NOVO
├── server.go     +   server_test.go     # NOVO
├── schedule.go                          # MOD — sem novos métodos; resources do provider montam payloads diferentes
├── application.go                       # MOD — ApplicationInput ganha ServerID
├── postgres.go                          # MOD — Postgres + PostgresInput ganham ServerID
├── mysql.go                             # MOD
├── mariadb.go                           # MOD
├── mongo.go                             # MOD
└── redis.go                             # MOD
```

`internal/provider/database_helpers.go` ganha `generateSSHKeyPair() (privatePEM, publicOpenSSH string, error)`.

## Configuração do provider

Nenhuma mudança.

## Recursos

### `dokploy_ssh_key`

| Atributo | Tipo | Notas |
|---|---|---|
| `organization_id` | string, **obrigatório**, ForceNew | `data.dokploy_organization.x.id`. Necessário porque a API exige e o usuário tem >1 org. |
| `name` | string, **obrigatório** | nome de exibição. |
| `public_key` | string, opcional + **computed** | formato OpenSSH. Se omitida, provider gera junto da privada. |
| `private_key` | string, opcional + **computed**, sensitive | PEM PKCS#1. Se omitida, provider gera. |
| `id` | string, **computed** | `sshKeyId`. |

**Geração client-side** (quando `private_key`/`public_key` omitidos):
- RSA 4096 via `crypto/rsa.GenerateKey`.
- Privada: `pem.Encode` com type `RSA PRIVATE KEY` e bytes de `x509.MarshalPKCS1PrivateKey`.
- Pública: `ssh.MarshalAuthorizedKey(ssh.NewPublicKey(pub))`, com comentário sufixo `" <name>\n"`.
- Geração só acontece no Create. Update: se usuário muda os valores, são enviados ao API; se omite após ter setado, mantém state (Optional+Computed).

### `dokploy_server`

| Atributo | Tipo | Notas |
|---|---|---|
| `name` | string, **obrigatório** | |
| `description` | string, opcional + **computed** | API exige o campo presente; provider sempre envia (default `""`). |
| `ip_address` | string, **obrigatório**, ForceNew | IP ou hostname. |
| `port` | number, opcional + **computed** | default `22`. Atualizável. |
| `username` | string, opcional + **computed** | default `root`. Atualizável. |
| `ssh_key_id` | string, **obrigatório**, ForceNew | `dokploy_ssh_key.x.id`. |
| `server_type` | string, opcional + **computed** | enum `"deploy"` ou `"build"`, default `"deploy"`. Atualizável. |
| `id` | string, **computed** | `serverId`. |
| `organization_id` | string, **computed** | herdado da `ssh_key_id` / API key. |

**Handshake SSH no create:** se falhar (chave pública não está em
`authorized_keys` da VM, IP errado, porta bloqueada, etc), o create da resource
falha com a mensagem do servidor. O `dokploy_ssh_key` referenciado não é
afetado.

### `dokploy_server_schedule`

Cron rodando em um worker remoto (`scheduleType: "server"`).

| Atributo | Tipo | Notas |
|---|---|---|
| `server_id` | string, **obrigatório**, ForceNew | `dokploy_server.x.id`. |
| `name` | string, **obrigatório** | |
| `cron_expression` | string, **obrigatório** | |
| `command` | string, **obrigatório** | |
| `shell_type` | string, opcional + **computed** | default `bash`. |
| `enabled` | bool, opcional + **computed** | default `true`. |
| `timezone` | string, opcional | |
| `id` | string, **computed** | `scheduleId`. |
| `app_name` | string, **computed** | nome interno gerado pelo Dokploy. |

Cliente reusa `internal/client/schedule.go` (sem mudanças no client) — o
resource monta `ScheduleInput{ScheduleType: "server", ServerID: ..., ...}`.

> **Pré-requisito:** o `ScheduleInput` em `internal/client/schedule.go` precisa
> de um campo `ServerID string json:"serverId,omitempty"` (já existe no struct
> `Schedule` de v0.3 — adicionado ao Input agora). Adição mecânica, não muda
> comportamento dos schedules existentes.

### `server_id` nos 6 resources existentes

Adicionado em `application`, `postgres`, `mysql`, `mariadb`, `mongo`, `redis`:

| Atributo | Tipo | Notas |
|---|---|---|
| `server_id` | string, opcional + **computed**, ForceNew | `dokploy_server.x.id`. Omitir = roda no host do Dokploy. Mudar = recria o recurso. |

**Mudanças nos client structs:** todos os 6 structs (`Application`, `Postgres`,
`Mysql`, `Mariadb`, `Mongo`, `Redis`) e seus `*Input` ganham
`ServerID *string json:"serverId,omitempty"`. O campo já é retornado pela API
(visto na key list de `application.one` no v0.1), mas as structs Go atuais não
o expõem.

A Tarefa 1 do plano verifica via probe se cada um dos 5 DBs realmente aceita
`serverId` no `<db>.create` (provável — padrão do Dokploy). Se algum não
aceitar, esse específico não ganha o atributo na v0.4, documentamos a
limitação e seguimos.

## Comportamento de erro

- **Handshake SSH falha** em `dokploy_server.create`: erro propagado como
  diagnóstico Terraform com a mensagem do servidor. Resource não fica no state.
- **`dokploy_ssh_key` deletado enquanto um `dokploy_server` o referencia:** a
  API provavelmente bloqueia. Se permitir, o server fica órfão com `ssh_key_id`
  apontando pra ID inexistente. Read detectará 404 e marcará drift. (A
  verificar na Tarefa 1.)
- **`server_id` aponta pra server inexistente** em
  application/postgres/etc.create: API rejeita; erro propagado.

## Drift, refresh e import

Padrão idêntico aos resources existentes:
- Read chama `<router>.one`. 404 → `IsNotFound` → remove do state.
- `terraform import dokploy_ssh_key.x <sshKeyId>` (e equivalentes).
- `dokploy_ssh_key`: import preenche `name`, `organization_id`, `public_key`,
  mas **não preenche `private_key`** — a API não retorna chaves privadas no
  read (verificar na Tarefa 1). Documentado: importar uma chave criada fora do
  Terraform implica que o state nunca terá a privada. Usuário pode rotar
  fornecendo nova `private_key`+`public_key` no config.

## Estratégia de testes

**Unitários (`internal/client`)** — TDD com `httptest`, sem rede:
- `sshkey_test.go`: Create/Get/Update/Delete + header `x-api-key` + serialização do corpo.
- `server_test.go`: idem para `server.*`.
- `schedule_test.go` (existing): adicionar caso `scheduleType: "server"`.

**Aceitação (`TF_ACC=1`) — sempre roda:**
- `TestAccSshKeyResource` — create (com geração client-side) + create (com chaves user-provided) + update do `name` + import + destroy. Verifica que `public_key` casa com formato OpenSSH e `private_key` casa com PEM `RSA PRIVATE KEY`.

**Aceitação — opt-in via env var (`DOKPLOY_TEST_SERVER_IP`):**
- `TestAccServerResource` — registra o server gerenciado, update do `description`, import, destroy.
- `TestAccServerScheduleResource` — cria schedule no server.
- `TestAccApplicationResource_OnServer` — variante do v0.1 test com `server_id` setado.
- `TestAccPostgresResource_OnServer` — variante do v0.2 test com `server_id` setado.

Cada um dos opt-in chama `t.Skip` no início se `DOKPLOY_TEST_SERVER_IP` não
estiver setado, com mensagem clara:

```
TestAccServerResource skipped: set DOKPLOY_TEST_SERVER_IP, _USER, _PORT,
and DOKPLOY_TEST_SERVER_PRIVATE_KEY to run server-dependent acceptance tests.
```

A privada usada pelos opt-in tests é fornecida pelo usuário (não gerada pelo
provider) — porque ela precisa ter a pública correspondente já no
`authorized_keys` da VM, o que é feito uma vez fora do CI.

## Distribuição

Sem mudanças no pipeline. Após merge:

```bash
git tag v0.4.0 && git push origin v0.4.0
```

`README.md` ganha 3 linhas:

```markdown
- `dokploy_ssh_key` — chave SSH (organização)
- `dokploy_server` — servidor remoto gerenciado
- `dokploy_server_schedule` — cron em servidor gerenciado
```

E os 6 resources existentes ganham menção a `server_id` na descrição
auto-gerada por `tfplugindocs`.

## Exemplo de uso (alvo)

```hcl
data "dokploy_organization" "current" {
  name = "Blitz IT Solutions"
}

resource "dokploy_ssh_key" "worker" {
  organization_id = data.dokploy_organization.current.id
  name            = "worker-key"
  # private_key/public_key omitidos → provider gera
}

output "worker_public_key" {
  value = dokploy_ssh_key.worker.public_key
  # cole isso em authorized_keys da VM remota antes de criar o dokploy_server.
}

resource "dokploy_server" "worker_sp" {
  name        = "worker-sp"
  description = "Worker São Paulo"
  ip_address  = "203.0.113.10"
  ssh_key_id  = dokploy_ssh_key.worker.id
  server_type = "deploy"
}

resource "dokploy_project" "app" {
  name = "minha-app"
}

# Database rodando no worker remoto
resource "dokploy_postgres" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  server_id      = dokploy_server.worker_sp.id   # << roda no worker
}

# Application rodando no worker remoto
resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "registry.example.com/my-api:1.0"
  server_id      = dokploy_server.worker_sp.id

  env = {
    DATABASE_URL = "postgres://app:${dokploy_postgres.db.database_password}@${dokploy_postgres.db.app_name}:5432/app"
  }
}

# Cron de manutenção no worker
resource "dokploy_server_schedule" "vacuum" {
  server_id       = dokploy_server.worker_sp.id
  name            = "pg-vacuum-weekly"
  cron_expression = "0 4 * * 0"
  command         = "docker exec ${dokploy_postgres.db.app_name} psql -U app -c 'VACUUM ANALYZE'"
  timezone        = "America/Sao_Paulo"
}
```

## Riscos e itens a verificar na implementação

1. **`<db>.create` aceita `serverId` em todos os 5 DBs?** Probar via empty-body
   POST para o Zod schema de cada um. Se algum não suportar, documentar e
   skipar.
2. **`sshKey.update` existe e permite trocar `private_key`/`public_key`?**
   Se não suportar, marcar ambos como ForceNew em vez de Optional+Computed.
3. **`sshKey.one` retorna `privateKey`?** Define a lógica de Read: sobrescreve
   do API ou preserva state (padrão "não sobrescreve sensitive").
4. **Delete verb para `sshKey` e `server`** — provavelmente `.remove` (padrão
   do Dokploy), mas confirmar no probe.
5. **Server.create faz handshake SSH síncrono ou só registra metadados?** Se
   for síncrono, erro de SSH bloqueia o create. Se for assíncrono, o status
   pode aparecer depois. Verificar e ajustar erro/timeout no resource.
6. **Schedule.update aceita mudança de `scheduleType`?** Provavelmente não
   (faria sentido proibir trocar entre application/server/dokploy-server) —
   garantir comportamento ForceNew implícito (resources são diferentes; o
   usuário não muda o type via Terraform, ele muda o tipo de resource).
