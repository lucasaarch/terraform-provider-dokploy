# Dokploy Terraform Provider — Design (v1)

**Data:** 2026-05-22
**Módulo Go:** `github.com/lucasaarch/terraform-provider-dokploy`
**Provider publicado:** `lucasaarch/dokploy`

## Objetivo

Criar um provider Terraform para o [Dokploy](https://dokploy.com) que permita
automatizar, de forma declarativa, uma infraestrutura inteira: desde a
organização até uma aplicação Docker no ar com domínio.

## Escopo da v1

Quatro recursos gerenciáveis + um data source, cobrindo o grafo completo de
deploy:

```
dokploy_organization (data source, somente leitura)
        │ referenciado por
        ▼
dokploy_project → dokploy_environment
                → dokploy_application (Docker) → dokploy_domain
```

**Sobre a organização:** a API key do Dokploy é amarrada a uma organização
existente — não é possível criar, alterar ou destruir organizações via API.
Logo, `dokploy_organization` é um **data source** (somente leitura) que expõe a
organização da API key. Os projetos são criados automaticamente nessa
organização.

**Fora de escopo na v1** (versões futuras): bancos de dados (Postgres/MySQL/
MariaDB/Mongo/Redis), Compose services, Servers, Registries, integração com
providers Git (GitHub/GitLab/etc.), backups, schedules. A `dokploy_application`
na v1 suporta **apenas origem Docker** (imagem de registry) — não suporta build
a partir de repositório Git.

## Decisões de arquitetura

### Abordagem: provider escrito à mão sobre `terraform-plugin-framework`

Usa o [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework)
(sucessor do SDKv2). O provider é escrito manualmente, não gerado.

A geração de código a partir do `/swagger` do Dokploy foi descartada: a API do
Dokploy é estilo RPC (`application.create`, `application.deploy`), não CRUD
REST, e ferramentas de codegen produzem código ruim nesse cenário. O `/swagger`
permanece como **referência** para mapear endpoints e tipos durante a
implementação.

### Duas camadas

- **`internal/client`** — não conhece Terraform. Fala HTTP com a API do
  Dokploy, expõe métodos tipados e structs de request/response. Testável
  isoladamente com `httptest`.
- **`internal/provider`** — conhece Terraform (schemas, state, plan). Traduz
  entre o state do Terraform e o cliente. Não faz HTTP direto.

## Estrutura do repositório

```
terraform-provider-dokploy/
├── main.go                       # entrypoint, registra o provider
├── internal/
│   ├── provider/
│   │   ├── provider.go            # config do provider (endpoint, api_key)
│   │   ├── organization_data_source.go
│   │   ├── project_resource.go
│   │   ├── environment_resource.go
│   │   ├── application_resource.go
│   │   ├── domain_resource.go
│   │   └── *_test.go              # testes de aceitação (TF_ACC)
│   └── client/
│       ├── client.go              # cliente HTTP fino + tratamento de erro
│       ├── organization.go        # chamadas organization.*
│       ├── project.go             # chamadas project.*
│       ├── environment.go         # chamadas environment.*
│       ├── application.go         # chamadas application.* + deploy
│       ├── domain.go              # chamadas domain.*
│       └── *_test.go              # testes unitários (httptest)
├── examples/                      # exemplos .tf para docs do Registry
├── docs/                          # docs geradas (terraform-plugin-docs)
├── templates/                     # templates de docs
├── .goreleaser.yml                # build/assinatura de releases
├── .github/workflows/             # CI (lint, test) + release
└── LICENSE                        # MPL-2.0
```

## Configuração do provider

Bloco `provider "dokploy"`:

| Atributo | Tipo | Notas |
|---|---|---|
| `endpoint` | string | URL base da instância Dokploy (ex: `https://dokploy.exemplo.com`). Env: `DOKPLOY_ENDPOINT`. |
| `api_key` | string, `sensitive` | Token enviado no header `x-api-key`. Env: `DOKPLOY_API_KEY`. |

Ao menos um dos dois (atributo ou env var) deve estar presente para cada campo;
caso contrário o provider emite erro de configuração.

## Cliente da API (`internal/client`)

A API do Dokploy é RPC sobre HTTP: mutações são `POST` com corpo JSON
(`project.create`); leituras são `GET` com query params
(`project.one?projectId=...`).

**`client.go` — núcleo:**

- `New(endpoint, apiKey string) *Client` — guarda base URL + token.
- `do(ctx, method, path, body, query, &out) error` — monta a request, injeta
  o header `x-api-key` e `Accept: application/json`, decodifica a resposta.
- **Erros:** status ≥ 400 vira `*APIError` tipado (status code + mensagem do
  corpo). `IsNotFound(err)` detecta `404` — usado pela lógica de drift.
- Timeout configurável; `context.Context` propagado em todas as chamadas.

**Arquivos por recurso** expõem métodos tipados:

- `organization.go`: `ListOrganizations` (lê `organization.all`; a API key
  enxerga exatamente uma organização)
- `project.go`: `CreateProject`, `GetProject`, `UpdateProject`, `DeleteProject`
- `environment.go`: `CreateEnvironment`, `GetEnvironment`, `UpdateEnvironment`,
  `DeleteEnvironment`
- `application.go`: `CreateApplication`, `GetApplication`, `UpdateApplication`,
  `DeleteApplication`, `SaveDockerProvider`, `SaveEnvironment`,
  `DeployApplication`, `GetApplicationStatus`
- `domain.go`: `CreateDomain`, `GetDomain`, `UpdateDomain`, `DeleteDomain`

**Verificação na implementação:** os nomes de endpoint e formatos de payload
foram parcialmente verificados contra a instância real (`project.all`,
`organization.all`) e o restante é confirmado no primeiro passo do plano. Fatos
já confirmados: a API responde em `<endpoint>/api/<router>.<método>`; projeto
tem `projectId`, `organizationId`, `environments[]`; environment tem
`environmentId`, `name`, `isDefault` (bool — `true` no environment de produção);
organização (de `organization.all`) tem `id`, `name`, `slug`.

## Data sources

### `dokploy_organization`

Data source somente leitura. Expõe a organização à qual a API key pertence.
A API key vê exatamente uma organização, então o data source não exige
argumentos de entrada — lê `organization.all` e retorna a organização única.

| Atributo | Tipo | Notas |
|---|---|---|
| `id` | string, **computed** | identificador da organização |
| `name` | string, **computed** | nome da organização |
| `slug` | string, **computed** | slug da organização (pode ser vazio) |

## Recursos

### `dokploy_project`

| Atributo | Tipo | Notas |
|---|---|---|
| `name` | string, **obrigatório** | |
| `description` | string, opcional | |
| `organization_id` | string, **computed** | organização do projeto — preenchida automaticamente (a API key define a organização; não é configurável) |
| `production_env` | map(string), opcional | variáveis de ambiente compartilhadas do environment `production` (criado automaticamente com o projeto) |
| `id` | string, **computed** | `projectId` |
| `production_environment_id` | string, **computed** | ID do environment `production` (o environment com `isDefault = true`) |

**Nota sobre o environment `production`:** todo projeto Dokploy nasce com um
environment `production`. Seu nome e descrição são fixos (não editáveis pela
API) — apenas variáveis de ambiente podem ser definidas. Por isso o `production`
é gerenciado como faceta do `dokploy_project` (atributo `production_env`), não
pelo recurso `dokploy_environment`.

### `dokploy_environment`

Gerencia **apenas environments customizados** (adicionais ao `production`).

| Atributo | Tipo | Notas |
|---|---|---|
| `project_id` | string, **obrigatório** | referencia `dokploy_project.x.id`. ForceNew. |
| `name` | string, **obrigatório** | ex: `staging`, `dev` |
| `description` | string, opcional | |
| `env` | map(string), opcional | variáveis de ambiente compartilhadas |
| `id` | string, **computed** | `environmentId` |

Não declarar um `dokploy_environment` chamado `production` — esse environment é
automático e gerenciado via `dokploy_project.production_env`.

### `dokploy_application`

Aplicação com origem **Docker** (imagem de registry).

| Atributo | Tipo | Notas |
|---|---|---|
| `environment_id` | string, **obrigatório** | referencia `dokploy_project.x.production_environment_id` **ou** `dokploy_environment.x.id` |
| `name` | string, **obrigatório** | |
| `description` | string, opcional | |
| `docker_image` | string, **obrigatório** | ex: `nginx:1.27` |
| `registry_url` | string, opcional | registry privado |
| `registry_username` | string, opcional, `sensitive` | |
| `registry_password` | string, opcional, `sensitive` | |
| `env` | map(string), opcional | variáveis de ambiente da aplicação |
| `id` | string, **computed** | `applicationId` |
| `app_name` | string, **computed** | nome interno gerado pelo Dokploy |
| `status` | string, **computed** | status do último deploy |

Suporta o bloco padrão `timeouts` (`create`, `update`), default 10 minutos.

### `dokploy_domain`

| Atributo | Tipo | Notas |
|---|---|---|
| `application_id` | string, **obrigatório** | referencia `dokploy_application.x.id` |
| `host` | string, **obrigatório** | hostname, ex: `app.exemplo.com` |
| `path` | string, opcional | default `/` |
| `port` | number, opcional | porta do container para rotear |
| `https` | bool, opcional | default `false` |
| `certificate_type` | string, opcional | `none` ou `letsencrypt`, default `none` |
| `id` | string, **computed** | `domainId` |

## Fluxo de deploy automático (`dokploy_application`)

Criar e configurar uma aplicação e fazer o deploy são passos separados na API.
A `dokploy_application` os orquestra numa única operação.

**Create:**

1. `application.create` → obtém `applicationId`
2. `application.saveDockerProvider` → imagem + credenciais de registry
3. `application.saveEnvironment` → variáveis de ambiente
4. `application.deploy` → dispara o build/deploy (assíncrono)
5. **Polling:** consulta `application.one` lendo o campo de status até chegar
   em `done` (sucesso) ou `error` (falha)

**Update:** aplica as mudanças nos endpoints relevantes, redispara
`application.deploy` e refaz o polling. Mudança em `docker_image`, `env` ou
credenciais de registry dispara novo deploy.

**Falha de deploy:** se o status virar `error`, o `terraform apply` falha com
mensagem clara apontando a falha e indicando onde ver os logs no painel do
Dokploy. O recurso **permanece no state** (foi criado) — um próximo apply pode
corrigir, sem deixar recurso órfão.

**Timeout:** o bloco `timeouts` controla o limite de polling. O polling
respeita o `context` do Terraform (cancela se o usuário interromper).

**Domínios:** `dokploy_domain` é criado após a aplicação existir. `domain.create`
apenas registra o domínio; o roteamento do Traefik é aplicado pelo Dokploy.
Não há polling no domínio.

## Drift, refresh e import

**Read / refresh** — cada recurso tem um Read que chama o endpoint `*.one`:

- `404` → `IsNotFound(err)` detecta, o recurso é removido do state; o próximo
  plan recria.
- Outros erros → propagados como diagnóstico de erro do Terraform.
- Valores alterados pelo painel do Dokploy aparecem como diff no `plan`.

**Valores sensíveis não retornados pela API** — `registry_password`
provavelmente não volta no `application.one`. O Read **não sobrescreve** esse
campo com o valor da API; mantém o do state/config. Consequência documentada:
drift em `registry_password` feito fora do Terraform não é detectado.

**Import** — os 4 recursos gerenciáveis suportam `terraform import` pelo ID
nativo do Dokploy. O Read preenche todos os atributos a partir do `*.one`, então
o import não exige informar atributos pai à mão. (`dokploy_organization` é um
data source — não tem import.)

```
terraform import dokploy_project.app       <projectId>
terraform import dokploy_environment.stg   <environmentId>
terraform import dokploy_application.api   <applicationId>
terraform import dokploy_domain.web        <domainId>
```

## Estratégia de testes

**Testes unitários (`internal/client`)** — desenvolvidos por TDD, sem Dokploy
real. Servidor `httptest` simula a API: verifica path correto, header
`x-api-key`, serialização do corpo, e a interpretação de status codes por
`IsNotFound`/`APIError`. Rodam em qualquer CI.

**Testes de aceitação (`TF_ACC=1`)** — contra a instância real do usuário,
configurada por `DOKPLOY_ENDPOINT` e `DOKPLOY_API_KEY`. Usam o `helper/resource`
do framework de testes do Terraform. Para cada recurso:

- `create` + `Read` (verifica atributos computed)
- `update` (muda um atributo, confirma a aplicação)
- `import` (importa e confirma que o state bate)
- `CheckDestroy` (confirma remoção no Dokploy após o destroy)

Um teste end-to-end monta o grafo completo (data source `dokploy_organization`
→ project → environment → application Docker → domain) com deploy real e
destrói tudo ao final.

**Separação:** testes unitários rodam em todo PR; testes de aceitação rodam sob
demanda / em job protegido (consomem recursos reais e precisam dos segredos).

## Distribuição e CI

- **GoReleaser** (`.goreleaser.yml`) — builds cross-platform (linux/darwin/
  windows, amd64/arm64), `SHASUMS` e assinatura GPG, no formato exigido pelo
  Terraform Registry.
- **GitHub Actions:**
  - PR/push → `golangci-lint` + testes unitários + `go build`
  - tag `v*` → GoReleaser publica um GitHub Release com binários assinados +
    `terraform-registry-manifest.json`
- **Docs** — `terraform-plugin-docs` gera `docs/` a partir dos schemas +
  exemplos em `examples/`. Verificado no CI (docs desatualizadas quebram o
  build).
- **Publicação no Registry** — registrar o provider em registry.terraform.io
  no namespace `lucasaarch`, com a chave GPG pública cadastrada. Nome publicado:
  `lucasaarch/dokploy`.
- **Licença** — MPL-2.0 (padrão dos providers Terraform).

## Exemplo de uso (alvo)

```hcl
provider "dokploy" {
  endpoint = "https://dokploy.exemplo.com"
  # api_key via env DOKPLOY_API_KEY
}

data "dokploy_organization" "current" {}

resource "dokploy_project" "app" {
  name = "minha-app"
  production_env = {
    LOG_LEVEL = "info"
  }
}

resource "dokploy_environment" "staging" {
  project_id  = dokploy_project.app.id
  name        = "staging"
  description = "Ambiente de testes"
  env = {
    LOG_LEVEL = "debug"
  }
}

resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "nginx:1.27"
  env = {
    PORT = "8080"
  }
}

resource "dokploy_domain" "web" {
  application_id   = dokploy_application.api.id
  host             = "api.exemplo.com"
  port             = 8080
  https            = true
  certificate_type = "letsencrypt"
}
```

## Riscos e itens a verificar na implementação

1. **Nomes de endpoints e payloads de mutação** — `project.all` e
   `organization.all` já confirmados. Falta confirmar `*.create`/`*.update`/
   `*.remove` de project, environment, application, domain e os endpoints
   `application.saveDockerProvider`/`saveEnvironment`/`deploy`.
2. **Router `environment.*`** — confirmar que existe e seu formato.
3. **Campo de status do deploy** — confirmar os valores possíveis
   (`idle`/`running`/`done`/`error` ou outros) e qual endpoint os expõe.
   `applicationStatus` já visto em `project.all` com valor `done`.
4. **`registry_password` no read** — confirmar se a API retorna ou omite o
   campo, ajustando a lógica de drift conforme.
