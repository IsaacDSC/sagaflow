# sagaflow

<p align="center">
  <img src="docs/assets/sagaflow.png" alt="SagaFlow - orquestracao de transacoes distribuidas" width="900" />
</p>

## Visao geral
O `sagaflow` executa sagas/transacoes distribuidas a partir de regras persistidas em banco.

Fluxo principal:
1. Voce cria/atualiza uma regra via API (`PUT /api/v1/rule`).
2. Inicia uma transacao chamando (`POST /api/v1/transaction/rule/{id}`).
3. O orquestrador chama os "servicos de transacao" (via HTTP) e, se houver falha, persiste o estado e tenta executar rollback (direto e/ou via tarefa de background).

## Arquitetura (Mermaid)

Tres fluxos separados: cadastro da regra, execucao da transacao e retry de rollback em background.

### 1. Criar regra

```mermaid
flowchart LR
  classDef client fill:#E3F2FD,stroke:#1976D2,color:#0D47A1,stroke-width:1px;
  classDef api fill:#E8F5E9,stroke:#43A047,color:#1B5E20,stroke-width:1px;
  classDef handler fill:#E1F5FE,stroke:#0288D1,color:#01579B,stroke-width:1px;
  classDef db fill:#FFEBEE,stroke:#E53935,color:#B71C1C,stroke-width:1px;
  classDef task fill:#FFF3E0,stroke:#FB8C00,color:#E65100,stroke-width:1px;
  classDef mem fill:#F3E5F5,stroke:#8E24AA,color:#4A148C,stroke-width:1px;

  C[Cliente]:::client --> A[API HTTP]:::api
  A --> H[Handler PUT rule]:::handler
  H --> PG[Postgres rules]:::db
  PG --> TK[Task RefreshRules]:::task
  TK --> MEM[MemStore regras]:::mem
```

A regra e persistida no banco; em seguida a task periodica recarrega todas as regras na memoria para o orquestrador consultar sem ir ao Postgres a cada request.

### 2. Executar transacao

```mermaid
flowchart TB
  classDef client fill:#E3F2FD,stroke:#1976D2,color:#0D47A1,stroke-width:1px;
  classDef api fill:#E8F5E9,stroke:#43A047,color:#1B5E20,stroke-width:1px;
  classDef orch fill:#FFF3E0,stroke:#FB8C00,color:#E65100,stroke-width:1px;
  classDef mem fill:#F3E5F5,stroke:#8E24AA,color:#4A148C,stroke-width:1px;
  classDef work fill:#E0F7FA,stroke:#00838F,color:#006064,stroke-width:1px;
  classDef ext fill:#F1F8E9,stroke:#558B2F,color:#33691E,stroke-width:1px;
  classDef db fill:#FFEBEE,stroke:#E53935,color:#B71C1C,stroke-width:1px;

  C[Cliente]:::client --> A[API HTTP]:::api
  A --> O[Orchestrator]:::orch
  O --> M[MemStore busca regra]:::mem
  O --> X[Executa steps]:::work
  X --> S[Servicos externos HTTP]:::ext
  X -->|falha| R[Executa rollback]:::work
  R --> S
  R -->|nao finalizou OK| P[Postgres pendencias]:::db
```

O payload recebido na API vira corpo JSON nas chamadas HTTP. Se algum passo de transacao falhar, o orquestrador dispara rollback; se o rollback nao concluir com sucesso, o estado e gravado no Postgres para a task de background.

### 3. Background: retry de rollback

```mermaid
flowchart LR
  classDef task fill:#EDE7F6,stroke:#5E35B1,color:#311B92,stroke-width:1px;
  classDef orch fill:#FFF3E0,stroke:#FB8C00,color:#E65100,stroke-width:1px;
  classDef db fill:#FFEBEE,stroke:#E53935,color:#B71C1C,stroke-width:1px;
  classDef work fill:#E0F7FA,stroke:#00838F,color:#006064,stroke-width:1px;
  classDef ext fill:#F1F8E9,stroke:#558B2F,color:#33691E,stroke-width:1px;

  T[Task FailRollbacks]:::task --> O[Orchestrator rollback]:::orch
  O --> PG[Postgres pendencias]:::db
  O --> H[HTTP rollback paralelo]:::work
  H --> S[Servicos externos]:::ext
  O --> U[Atualiza status/retries]:::db
```

A task roda em intervalo configuravel, busca transacoes em estado de falha no rollback (dentro do limite de retries) e tenta novamente chamar os endpoints de rollback em paralelo.

## Endpoints
- `GET /ping`
- `PUT /api/v1/rule`
  - Body: JSON com `name`, `transactions`, `rollback`, (opcional) `transforms` e `configs`.
  - Retorno: a propria regra com um `id` gerado/atualizado.
- `POST /api/v1/transaction/rule/{id}`
  - Body: JSON arbitrario (vira o payload enviado como body das chamadas HTTP dos servicos).
  - Retorno em sucesso: `202 Accepted` com `tx_id` (no corpo) e header `x-transaction-id`.

### API Limits
- Max timeout 2m for all transaction
- Method supported in transforms: GET
- Method supported in transactions/rollback: POST | PUT | PATCH | DELETE

Observacao: o campo `transforms` existe na regra, mas o fluxo atual do orquestrador nao executa transforms.

## Como a transacao funciona (resumo)
1. A API cria um `TransactionID` e chama o `Orchestrator.Transaction(...)`.
2. O orquestrador busca a regra em `MemStore` (recarregado periodicamente a partir do Postgres).
3. Ele executa `transactions`:
   - `configs.parallel: true`: chama em paralelo
   - `configs.parallel: false`: chama sequencialmente
4. Se alguma chamada de transacao falhar:
   - o orquestrador tenta executar rollback (via `rollback` da regra),
   - se o rollback nao conseguir finalizar com sucesso, o orquestrador persiste no Postgres uma transacao com status `failed_execute_rollback` para a tarefa de background continuar.
5. A tarefa `FailRollbacks` roda periodicamente, procura transacoes com `failed_execute_rollback` e tenta executar rollback em paralelo, atualizando status/retries.

## Como testar com Docker + curl

### 1. Subir os containers
Esse compose sobe:
- `postgres` + migrations (para persistir regras e transacoes),
- `sagaflow` (API em `:3001`),
- `example` (servidores mock nos `:3002` e `:3003`, para simular servicos externos).

```sh
docker compose --profile sagaflow --profile example up -d --build
```

### 2. Verificar healthcheck
```sh
curl -i http://localhost:3001/ping
```

### 3. Criar regra
Use os exemplos de `example/ex.md`. Primeiramente, crie a regra:

```sh
curl -X PUT \
  http://localhost:3001/api/v1/rule \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  -d @example/put_rule.json
```

Repare que o retorno vai incluir o campo `id`. Para garantir que a chamada abaixo usa a regra correta, substitua `{id}` pelo `id` retornado.

### 4. Iniciar transacao
Exemplo (use o `id` da etapa anterior):

```sh
curl -X POST \
  http://localhost:3001/api/v1/transaction/rule/{id} \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  -d @example/seller.json
```

O comando acima segue o mesmo formato de `example/ex.md` (a diferenca e apenas trocar `{id}` pelo `id` retornado no `PUT /api/v1/rule`).

Se voce quiser usar exatamente o comando de `example/ex.md` (observacao: ajuste o `rule id` conforme a regra que voce cadastrou):

```sh
curl -X POST \
  http://localhost:3001/api/v1/transaction/rule/9a7b94fb-db93-4fb2-99c6-271f61af48e8 \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  -d @example/seller.json
```

Se quiser testar o fluxo de nao-paralelo, crie uma regra alternativa com:
```sh
curl -X PUT \
  http://localhost:3001/api/v1/rule \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  -d @example/put_rule_non_parallel.json
```

E depois dispare a transacao usando o `id` retornado para aquela regra.

### Notas sobre os mocks do `example`
Os servidores mock em `example` retornam `429 Too Many Requests` por algumas chamadas (limitado por um contador interno), entao e esperado que a saga:
- re-tente chamadas (config `max_retry` da regra),
- e eventualmente execute rollback/persistencia se a falha nao for recuperada.