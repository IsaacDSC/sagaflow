# Especificação: SDK Go para gqueue (`pkg/gqueue`)

Este documento descreve a abordagem proposta para um SDK em Go que conversa com o serviço de fila **gqueue** via HTTP. Não descreve implementação de código; apenas arquitetura, convenções, superfícies públicas e padrões (configuração idiomática, cliente HTTP e *builders*).

## Objetivos

- Encapsular os endpoints REST do gqueue com tipos fortes, erros claros e defaults sensatos.
- Expor um **cliente configurável** (URL base, timeout, transporte, headers globais) compatível com o ecossistema Go (`context.Context`, `*http.Client` injetável).
- Permitir construção fluente de requisições complexas via **builder**, sem obrigar struct literals grandes nos call sites.
- Manter o pacote enxuto, testável e sem dependências pesadas além da stdlib (salvo decisão futura explícita, por exemplo para retry/metrics).

## Escopo inicial

| Operação | Método e caminho (relativo à base URL) | Uso no SDK |
|----------|----------------------------------------|------------|
| Criar ou atualizar evento + consumidores | `PUT /api/v1/event/consumer` | Método no cliente, por exemplo `UpsertEventConsumer` |
| Publicar mensagem (pub/sub) | `POST /api/v1/pubsub` | Método no cliente, por exemplo `Publish` |

Autenticação nos exemplos: **HTTP Basic** (`Authorization: Basic …`). O SDK deve suportar isso de primeira classe via config, sem hardcodar credenciais.

## Layout do pacote sugerido

```
pkg/gqueue/
  client.go          # Client, NewClient, opções, execução de requests
  config.go          # Config / ClientOption (functional options)
  types_event.go     # structs JSON para PUT event/consumer
  types_pubsub.go    # structs JSON para POST pubsub
  builder_event.go   # builders para montar UpsertEventConsumer
  builder_pubsub.go  # builders para montar Publish
  errors.go          # tipos de erro (API, decode, rede)
```

Alternativa: um subpacote `pkg/gqueue/internal/httputil` apenas se a duplicação de serialização/`Do` justificar; na primeira versão, prefira um único pacote `gqueue` para API pública simples.

## Configuração idiomática (HTTP)

### Princípios

1. **URL base obrigatória** (por exemplo `http://localhost:8080`). Path dos endpoints deve ser relativo a ela para facilitar ambientes diferentes sem duplicar paths.
2. **`*http.Client` opcional**: se nil, o SDK cria um cliente com timeout default; se não nil, usa o fornecido (permitindo `Transport` custom, TLS, connection pooling compartilhado com outros clientes).
3. **`context.Context` em todas as chamadas públicas** que disparam rede, por exemplo `client.Publish(ctx, …)`.
4. **Headers padrão**: `Content-Type: application/json`, `Accept: application/json` (quando fizer sentido), mais headers injetados por options (tracing, tenant, etc.).

### Functional options (`ClientOption`)

Padrão idiomaticamente Go para extender config sem explosão de parâmetros em `NewClient`:

- `WithHTTPClient(*http.Client)` — cliente HTTP compartilhado.
- `WithBaseURL(string)` — validar esquema e normalizar (sem barra final duplicada nos paths).
- `WithBasicAuth(username, password string)` — preenche `req.SetBasicAuth` em toda requisição do SDK (ou guarda credencial de forma segura só durante o `Do`).
- `WithRequestHook(func(*http.Request) error)` — ponto único para assinaturas extras ou substituição futura de Basic por Bearer.
- `WithUserAgent(string)` — opcional.

`NewClient(opts ...ClientOption) (*Client, error)` deve validar `BaseURL` e falhar cedo com erro claro.

### Segurança e observabilidade (especificação)

- Não logar corpo de request/response com dados sensíveis por default.
- Erros de API devem preservar **status HTTP** e, quando possível, **corpo** bruto ou struct de erro do gqueue (se documentado); caso contrário, `body snippet` limitado em tamanho.

## Modelagem dos payloads (tipos)

Os exemplos abaixo orientam os campos das structs; nomes em Go em **PascalCase** com tags `json` em **snake_case** para alinhar ao JSON do serviço.

### PUT `/api/v1/event/consumer`

- **Raiz**
  - `name` (`string`) — ex.: `payment.charged`
  - `type` (`string`) — ex.: `external`
  - `option` (struct) — opções de fila
  - `consumers` (`[]Consumer`)

- **`option`**
  - `wq_type` — ex.: `low_throughput`
  - `max_retries` — inteiro
  - `retention` — string de duração como no serviço (ex.: `168h`); no SDK pode ser `string` na v1; opcionalmente um tipo que serializa para o mesmo formato se o contrato for estável.

- **`consumers[]`**
  - `service_name`, `type`, `host`, `path`
  - `headers` — `map[string]string` ou `http.Header`-like que serializa como objeto JSON simples.

Campos opcionais no contrato real do gqueue devem usar ponteiros ou `omitempty` conforme semântica (evitar enviar zero values que mudem comportamento do servidor).

### POST `/api/v1/pubsub`

- `service_name`
- `event_name`
- `data` — `map[string]any` ou tipo genérico documentado (`json.RawMessage` para payload opaco); a escolha afeta ergonomia: `map[string]any` é simples, `json.RawMessage` evita round-trip de tipos arbitrários.
- `metadata` — map ou struct com campos conhecidos (`correlation_id`, etc.) + extensão se necessário.

## Padrão Builder

*Builders* não substituem os tipos JSON; eles **constroem** esses tipos e validam invariantes antes da chamada HTTP.

### Builder para publicação (`Publish`)

Fluxo sugerido:

1. `gqueue.NewPublishBuilder(serviceName, eventName)` retorna um builder.
2. Métodos encadeáveis:
   - `WithData(map[string]any)` ou `WithDataFrom(struct any)` (via `json.Marshal` interno, com documentação de erro se falhar).
   - `WithMetadata(key, value string)` e/ou `WithCorrelationID(uuid string)`.
   - `Validate() error` — obrigatório ou chamado implicitamente em `Build()`.
3. `Build()` retorna um `PublishInput` imutável (struct de valor) ou o próprio tipo esperado por `Client.Publish`.
4. `Client.Publish(ctx, input)` executa o POST.

Vantagens: call sites legíveis; validação centralizada; fácil adicionar campos novos sem quebrar assinaturas antigas (além de deprecar métodos se preciso).

### Builder para evento/consumidor (`UpsertEventConsumer`)

1. `gqueue.NewEventConsumerBuilder(eventName string)` ou nome equivalente.
2. Encadear:
   - `WithEventType("external")`
   - `WithOption(...)` — sub-builder ou struct `Option`: `WQType`, `MaxRetries`, `Retention`.
   - `AddConsumer(...)` — aceita `Consumer` pronto ou sub-builder `NewConsumerBuilder()` com `WithHost`, `WithPath`, `WithServiceName`, `WithConsumerType`, `WithHeader`.
3. `Build()` → `UpsertEventConsumerInput`.

Sub-builders para `Consumer` evitam structs aninhadas longas e refletem o exemplo `curl`.

### Imutabilidade e erros de builder

- `Build()` retorna `(T, error)` se qualquer passo puder falhar (ex.: URL inválida no consumer); alternativa: acumular erro interno no builder (padrão menos idiomático em Go, mas usado em algumas libs). **Recomendação:** `(T, error)` em `Build()` para ser explícito.
- Documentar quais combinações de campos são inválulas se o gqueue impor regras não expressas no JSON.

## Cliente: contrato dos métodos

Sugestão de assinaturas (nomes finais a definir na implementação):

```go
func (c *Client) UpsertEventConsumer(ctx context.Context, input UpsertEventConsumerInput) error
func (c *Client) Publish(ctx context.Context, input PublishInput) error
```

Variantes:

- Retornar corpo de resposta tipado se o servidor devolver IDs ou confirmações (`(*UpsertResult, error)`).
- Sobrecargas não existem em Go; usar structs de resultado quando necessário.

### Execução HTTP interna

- Um método privado `doJSON(ctx, method, path, reqBody, respBody)` que:
  - monta `http.NewRequestWithContext`
  - aplica headers default + Basic auth da config
  - `json.Marshal` / `json.NewDecoder` com `DisallowUnknownFields()` **opcional** (decisão: ligar se estabilidade do contrato for alta; senão, documentar que campos desconhecidos são ignorados).
- Tratar `4xx/5xx` com tipo `APIError` contendo `StatusCode`, `Body []byte` ou string, e eventual `RequestID` se header existir.

## Testes (abordagem)

- **Testes de unidade** dos builders: `Build()` com casos válidos e inválidos; garantir JSON esperado com `json.Marshal` e `cmp` ou substring em golden files leves.
- **Testes de integração** opcionais com `httptest.Server` simulando gqueue: verificar método, path, headers `Authorization`, corpo JSON.
- Não acoplar testes a `localhost:8080` real.

## Versionamento e compatibilidade

- Manter mudanças de breaking explícitas (prefixed v2 module se no futuro o módulo Go exigir).
- Campos novos do servidor: preferir que structs exportadas usem `json` tolerante ou tipos extensíveis (`json.RawMessage` em seções opacas) para não quebrar clientes antigos.

## Fora do escopo da primeira versão (explícito)

- Retries com backoff, idempotency keys, e métricas — podem ser acrescentados via `RoundTripper` wrapper ou options depois.
- Cliente assíncrono ou worker — apenas chamadas síncronas com `context`.
- CLI — apenas biblioteca em `pkg/gqueue`.

## Referência rápida dos exemplos (contrato HTTP)

**PUT event/consumer** — corpo inclui `name`, `type`, `option` (`wq_type`, `max_retries`, `retention`) e `consumers[]` com `service_name`, `type`, `host`, `path`, `headers`.

**POST pubsub** — corpo inclui `service_name`, `event_name`, `data` (objeto), `metadata` (ex.: `correlation_id`).

Autenticação: header `Authorization: Basic …` conforme credenciais configuradas no cliente.

---

Este arquivo é a fonte da verdade para implementação futura do pacote; alterações no contrato do gqueue devem refletir aqui antes de mudar código público do SDK.
