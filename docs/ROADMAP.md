# ROADMAP — job-radar

Só a fase atual aparece em detalhe. As próximas ficam em uma linha.
Quando a Fase 0 fechar, a Fase 1 é expandida.

---

## Visão geral

| Fase   | Nome                          | Pronto quando                                                                                          |
| ------ | ----------------------------- | ------------------------------------------------------------------------------------------------------ |
| **0**  | Uma fonte só, ponta a ponta   | `docker compose up` coleta, o teste de integração passa e o Python enxerga o banco                     |
| 1      | Coleta de verdade             | Mais fontes atrás de `Source`, normalização, dedupe, rate limit, métricas — roda 7 dias sozinho        |
| 2      | Olhar o dado                  | EDA + boilerplate medido, política de localização, vagas PT-BR — 5 defeitos listados + corte de boilerplate com teste |
| 3a     | Extração no código            | Skills, anos exigidos, salário, idade da vaga; ranking TF-IDF — extrator acerta as skills do currículo |
| 3b     | Rotulagem e avaliação         | `radar label` (~100 vagas), conjunto de teste congelado, TF-IDF medido nele                            |
| 4a     | Embeddings                    | sentence-transformers + pgvector vs baseline — sabe qual ganha e por quanto, no conjunto congelado     |
| 4b     | Jev como extrator de features | Question set v1, `jev_answers` — concordância por pergunta e custo por mil vagas medidos; roda sem chave |
| 4c     | sklearn                       | Features do Jev + cosseno + features do código — três abordagens comparadas no conjunto congelado      |
| 5      | FastAPI e digest              | Primeiro digest chega no Discord sem você rodar nada à mão                                             |
| 6      | Kubernetes                    | `kubectl apply -k deploy/k8s` sobe tudo do zero                                                        |
| 7      | Fechamento                    | Outra pessoa roda seguindo só o README                                                                 |

**Mudanças aplicadas ao plano original (Sessão Zero):**

- Prometheus saiu da Fase 1 e foi para a 6, onde existe scraper de verdade. `runs` cobre observabilidade até lá.
- Currículo (`cv.md`) ganhou dono explícito: Fase 3a.
- Fase 0 passou a atravessar Python também, com um `analyzer` mínimo.

**Revisão de 2026-09-29 (sincronização com o `CLAUDE.md`):** "Etapa" virou "Fase"; Fase 3
dividida em 3a/3b e Fase 4 em 4a/4b/4c (Jev entra na 4b); "rodar 7 dias sozinho" voltou a
ser o critério da Fase 1; rotulagem concentrada na 3b.

---

## Fase 0 — Uma fonte só, ponta a ponta ✅ (fechada 2026-09-30)

**Objetivo de aprendizado:** fazer o caminho mais fino possível atravessar Go, Postgres
e Python, provando que a integração funciona **antes** de qualquer feature. Ao fim
desta fase, `docker compose up` numa máquina limpa coleta vagas reais e o Python
consegue enxergá-las.

### Tarefa 0.1 — Migrations válidas e identidade estável

**Objetivo:** o banco sobe do zero com as sete tabelas, as fontes semeadas e `jobs` com
identidade própria.

**Critério de aceite:** `goose up` seguido de `goose down` roda limpo nos dois sentidos,
partindo de um banco vazio.

**Conceitos novos:** migration reversível, seed idempotente, chave natural vs chave
substituta, `ON CONFLICT`.

**Onde ler:** [goose — Writing migrations](https://github.com/pressly/goose#migrations) ·
[PostgreSQL — INSERT ON CONFLICT](https://www.postgresql.org/docs/16/sql-insert.html#SQL-ON-CONFLICT)

- [x] **0.1.1** Consertar `002_add_sources.sql`: semear as fontes da Etapa 0–1, de forma
      que rodar duas vezes não exploda, com `down` correspondente.
      ✅ 2026-08-26 — semeou só `remoteok`, decisão consciente: fonte semeada sem coletor
      é dado que mente. Novas fontes ganham sua própria migration na Etapa 1.
- [x] **0.1.2** Migration `003`: dar a `jobs` a identidade estável que a decisão da
      §4.2 do SPEC exige.
      ✅ 2026-08-26 — `(source_id, external_id)` `NOT NULL` + `UNIQUE` composta, FK só para
      `sources`. Verificado contra 4 cenários em Postgres real.

### Tarefa 0.2 — O coletor grava no banco

**Objetivo:** transformar o `main.go` atual (que baixa e joga fora) num coletor que
persiste, é idempotente e deixa rastro.

**Critério de aceite:** rodar o coletor duas vezes seguidas contra a mesma resposta da
API não duplica nada; a segunda execução aparece em `runs` com contagem coerente.

**Conceitos novos:** upsert, idempotência, transação, invalidação de score,
`fmt.Errorf` com `%w`.

**Onde ler:** [pgx — Transactions](https://pkg.go.dev/github.com/jackc/pgx/v5#Conn.Begin) ·
[Go — wrapping errors](https://go.dev/blog/go1.13-errors)

- [x] **0.2.1** Fazer o `main.go` compilar e persistir em `raw_jobs` — idempotente.
      ✅ 2026-08-26 — verificado contra a API real: 99 → 99 → 100 (após alterar hash).
- [x] **0.2.2** Normalizar `raw_jobs` → `jobs`, respeitando a regra de invalidação de
      `scored_at` da §4.2.
      ✅ 2026-09-03 — upsert por `(source_id, external_id)` com `DO UPDATE`; `scored_at`
      zera via `CASE` sobre `raw_job_id`. 3 rodadas idempotentes: 213 → 213 → 213.
- [x] **0.2.3** Registrar toda execução em `runs`, inclusive as que falham.
      ✅ 2026-09-03 — `openRun`/`closeRun` com `defer` + resultado nomeado. Verificado em 4
      cenários: sucesso, API 404, fonte ausente em `sources` (`source_id` NULL) e 100/101
      itens falhando (`fetched=101`, `failed=100`).

### Tarefa 0.3 — Teste de integração com testcontainers

**Objetivo:** provar o comportamento contra um Postgres real, sem depender da rede nem
da API do Remote OK estar de pé.

**Critério de aceite:** `go test ./...` sobe um Postgres, roda as migrations, alimenta o
coletor com `testdata/remoteok.json` e afirma a idempotência — tudo sem internet.

**Conceitos novos:** testcontainers-go, `httptest.Server`, injeção de dependência
por parâmetro, teste table-driven.

**Onde ler:** [testcontainers-go — Postgres module](https://golang.testcontainers.org/modules/postgres/) ·
[net/http/httptest](https://pkg.go.dev/net/http/httptest)

- [x] **0.3.1** Teste que sobe Postgres em container e aplica as migrations.
      ✅ 2026-09-09 — `TestMain` com container por pacote, `teardown` devolvido junto do erro
      em todos os `return`, migrations por `goose.NewProvider` sobre `fs.Sub(embed.FS)`.
      `TestMigrations` afirma nos dois sentidos (falta e sobra); validado por 3 mutações:
      tabela extra, tabela ausente e seed removida — as três detectadas.
- [x] **0.3.2** Alimentar o coletor com `testdata/remoteok.json` servido localmente e
      afirmar a idempotência de ponta a ponta.
      ✅ 2026-09-14 — `httptest.Server` servindo o fixture, `Config` montado no teste,
      `TRUNCATE` no início e duas execuções de `run`. Afirma contagem absoluta (100/100),
      igualdade entre as duas execuções e `runs` (2 linhas, nenhuma com `error`).
      Validado por 4 mutações: 3 detectadas, 1 escapou (`ON CONFLICT DO NOTHING` de
      `raw_jobs` — é o D12/D21, falta asserção sobre `runs.failed`).

### Tarefa 0.4 — Tudo dentro do compose, incluindo o Python

**Objetivo:** `docker compose up` numa máquina limpa faz o sistema inteiro funcionar,
e o Python prova que enxerga o mesmo banco.

**Critério de aceite:** clonar em diretório vazio, exportar as variáveis, `docker compose up`
→ o coletor roda e o `analyzer` imprime quantas vagas estão pendentes de score.

**Conceitos novos:** build multi-stage, `depends_on` com `condition`, healthcheck,
rede do compose, imagem distroless/alpine.

**Onde ler:** [Docker — Multi-stage builds](https://docs.docker.com/build/building/multi-stage/) ·
[Compose — depends_on](https://docs.docker.com/reference/compose-file/services/#depends_on)

- [x] **0.4.1** Dockerfile multi-stage do `collector` + serviço no compose, esperando o
      Postgres ficar saudável.
      ✅ 2026-09-15 — `golang:1.26.1-alpine` → `alpine:3.23`, 935 MB → 36.9 MB;
      `.dockerignore` tirando `.env`, `.git` e o binário morto do contexto; compose
      constrói pela chave `build:` mantendo a tag `job-searcher:latest`; credenciais
      só por `${}` (4 pontos, healthcheck incluído); `condition: service_healthy`.
      Verificado: imagem apagada → `docker compose up` reconstrói, espera `Healthy` e
      coleta (run 32, `fetched=100 failed=0 error NULL`); `go run .` do terminal também
      volta a funcionar, com a senha existindo 1× no repositório.
- [x] **0.4.2** `analyzer` Python mínimo no compose: conecta no Postgres e imprime a
      contagem de `jobs` com `scored_at is null`. Nada além disso.
      ✅ 2026-09-29 — projeto `uv` com `psycopg[binary]`, só `DATABASE_URL` (sem dotenv),
      variável ausente e banco fora do ar saem com uma linha no stderr e `exit=1`;
      `depends_on: app: service_completed_successfully`. `ruff` e `mypy --strict` limpos.
      Verificado: `docker compose up` → `app` sai 0 às 23:14:20.09, `analyzer` sobe às
      23:14:20.44 e imprime 363 = `psql`.
- [x] **0.4.3** Máquina limpa de verdade: banco sem schema vira banco com schema sem
      ninguém rodar `make gup` à mão, e quem clona sabe quais variáveis exportar
      (`.env.example`, D6). Fecha o critério de aceite da Tarefa 0.4.
      ✅ 2026-09-30 — coletor aplica as migrations ao subir (`embed.FS` movido do teste para
      produção, `goose.NewProvider` sobre `*sql.DB` via `pgx/stdlib`, log por migration e
      `goose.PartialError` tratado). `.env.example` monta a `DATABASE_URL` por `${}` a partir
      das outras chaves. Verificado em clone em `/tmp/radar-clean`, sem imagem prévia:
      build das duas imagens, 3 migrations aplicadas, `analyzer` com contagem > 0; segundo
      `up` sem reaplicar. **Tarefa 0.4 fechada.**

### Tarefa 0.5 — Fechamento da Fase 0

**Objetivo:** pagar os débitos agendados para o fim da fase e deixar o README ensinando a
rodar o projeto.

**Critério de aceite:** nenhum débito com dono "fim da Fase 0" aberto; outra pessoa roda o
projeto lendo só o README.

- [x] **0.5.1** Débitos de Go do fim da fase: D20 (`runs.error` sem o contexto do erro),
      D18 (INFO por item) e D22 (variável morta + nomes).
      ✅ 2026-09-30 — `processSource` com `(err error)`: fonte `fake` grava em `runs.error`
      o mesmo `failed to get source id: ...` do `WARN` (run 45). Log por item em `Debug` +
      uma linha `source collected` por fonte: execução normal caiu de 100+ para 2 linhas.
- [x] **0.5.2** README: o que é, pré-requisitos, `.env.example`, `docker compose up`, como
      rodar os testes.
      ✅ 2026-09-30 — escrito pelo mentor a pedido. Teste do leitor feito pelo mentor num clone
      do commit `5baa59d`: `up --build` → 3 migrations, `fetched=100 failed=0`, `analyzer`
      conta 99; `go test` verde. Achado: `ruff` falha no clone (D27).
      **Fase 0 fechada.**

---

## 📍 Fase 1 — Coleta de verdade

**Objetivo de aprendizado:** sair de uma fonte para N fontes com formatos de API
diferentes, atrás de uma abstração que **nasce da dor da duplicação** (não antes dela),
e endurecer o coletor para rodar sozinho por 7 dias: timeout, `context`, falha visível,
limiar de erro, rate limit e agendamento.

**Pronto quando:** o coletor roda sozinho por 7 dias, coletando de pelo menos três fontes
atrás da interface `Source`, com dedupe, rate limit por fonte e toda execução medida em `runs`.

### Tarefa 1.1 — Segunda fonte e a interface `Source`

**Objetivo:** o coletor passa a trazer vagas do Remotive além do Remote OK, e a lógica
específica de cada fonte deixa de estar assada em `fetchData`/`models.go` e passa a viver
atrás de uma interface pequena, definida no consumidor.

**Critério de aceite:** com `remoteok` e `remotive` ligadas no `config.yaml`, uma coleta
grava linhas das duas fontes em `jobs` (`SELECT source_id, count(*) FROM jobs GROUP BY
source_id` mostra as duas, com `title` e `company` preenchidos); rodar duas vezes seguidas
não duplica; um teste de integração alimenta o fixture do Remotive e afirma que ele vira `jobs`.

**Conceitos novos:** interface definida no consumidor, polimorfismo em Go, mapeamento de
formatos heterogêneos para um modelo comum, seed de fonte nova como migration própria.

**Onde ler:** [Effective Go — Interfaces](https://go.dev/doc/effective_go#interfaces) ·
[Go blog — JSON and Go](https://go.dev/blog/json)

- [x] **1.1.1 / 1.1.2 / 1.1.3** — feitas juntas em 2026-10-03. O aluno pulou o passo "na marra"
      e foi direto à interface. Migration `004_add_remotive.sql` (idempotente, `down` só da própria
      linha); `internal/sources/` com `Source` + `RemoteOK`/`Remotive` + factory `For(name)`;
      `TestFetch` estendido às duas fontes (117/117/4), verde. **Critério de aceite da 1.1 atendido.**
      Débitos de desenho abertos na mesma data: D28 (interface no implementador, não no consumidor),
      D29 (transação por item decorativa + `defer` no `for`), D30 (erro de `compactJson` descartado).

### Tarefa 1.2 — Terceira fonte (Himalayas) ✅ (fechada 2026-10-03)

Himalayas entra pela mesma interface, provando que a abstração generaliza. Se doer, a
interface está errada e volta para a 1.1.2.

- [x] **1.2** — `internal/sources/himalayas.go` + migration `005_add_himalayas.sql` + config.
      A interface `Source` **não** precisou mudar (generalizou); quem dobrou foi o modelo `Job`:
      `locationRestrictions` veio como array e o aluno achatou com `strings.Join`. Terceiro
      formato de data (epoch, `time.Unix`). Teste 137/137, `runs`=6, `TestSources`=3. Paginação
      (`nextCursor`/`totalCount`) deixada de fora de propósito — só a 1ª página (20 vagas).
      Débito aberto: D33 (`Date`/`Location`/`Tags` mapeados mas não persistidos).

### Tarefa 1.3 — Rodar sozinho sem mentir *(resumo)*

Timeout de HTTP + `context` (D7), falha total vira `exit≠0` (D21), migration de
`finished_at` + limiar `failed/fetched` (D12), `defer`/panic (D11).

- [x] **1.3.1** Timeout e `context` (D7). ✅ 2026-10-05 — client com 30s fixos + orçamento por
      fonte (parâmetro do `run`) num contexto que cobre source id, fetch e gravação; `closeRun`
      com contexto próprio. `TestClientTimeout` (0.11s) prova o corte e o `runs.error` gravado;
      validado por mutação.
- [x] **1.3.2** Todas as fontes falhando → `exit≠0` (D21). ✅ 2026-10-05 — `errors.Join` dos erros
      por fonte quando nenhuma teve sucesso (zero ligadas incluso); `TestPartialFailure` +
      `errors.Is` no `TestClientTimeout`; validado por 2 mutações.
- [x] **1.3.3** Limiar `failed/fetched` (D12, parte 1). ✅ 2026-10-05 — fonte com mais de 50% dos
      itens falhando vira falha (contagens em `runs.error`); zero itens é sucesso.
      `TestLimiarValidation` table-driven (3 casos), validado por mutações com limiar 0 e 1.0.
- [ ] **1.3.4** A run fecha com a verdade: `finished_at` (D12, parte 2) e panic não grava sucesso (D11).

### Tarefa 1.4 — Rate limit e retry por fonte *(resumo)*

### Tarefa 1.5 — Deduplicação *(resumo)* — mesma vaga aparecendo em duas fontes.

### Tarefa 1.6 — Agendamento e as 7 dias *(resumo)* — cron/ticker; roda uma semana sozinho.

### Tarefa 1.7 — Fechamento da Fase 1 *(resumo)* — débitos restantes (D17, D19), README.

---

## Fases seguintes (resumo)

**Fase 2 — Olhar o dado.** EDA em Python, boilerplate medido, política de localização, vagas PT-BR. Sai com 5 defeitos concretos e uma função de corte de boilerplate testada.

**Fase 3a — Extração no código.** `cv.md`, skills por dicionário/regex, anos exigidos, salário, idade da vaga, ranking TF-IDF.

**Fase 3b — Rotulagem e avaliação.** `radar label` (~100 vagas), conjunto de teste congelado, TF-IDF medido nele.

**Fase 4a — Embeddings.** sentence-transformers + pgvector contra a baseline, no conjunto congelado.

**Fase 4b — Jev como extrator de features.** Question set v1, `jev_answers`, piloto com anotação manual, pesos manuais.

**Fase 4c — sklearn.** Features do Jev + cosseno + features do código; três abordagens comparadas.

**Fase 5 — FastAPI e digest.** Digest semanal no Discord chegando sozinho.

**Fase 6 — Kubernetes.** Imagens multi-stage/multi-arch, minikube.

**Fase 7 — Fechamento.** CI, painel de tendências, README verificado por outra pessoa.
