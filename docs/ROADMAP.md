# ROADMAP — job-radar

Só a fase atual aparece em detalhe. As próximas ficam em uma linha; cada uma é expandida quando
a anterior fecha.

---

## Visão geral

| Fase   | Nome                          | Pronto quando                                                                                          |
| ------ | ----------------------------- | ------------------------------------------------------------------------------------------------------ |
| 0 ✅   | Uma fonte só, ponta a ponta   | `docker compose up` coleta, o teste de integração passa e o Python enxerga o banco                     |
| 1 ✅   | Coleta de verdade             | Mais fontes atrás de `Source`, normalização, dedupe, rate limit, métricas — roda 7 dias sozinho        |
| **2**  | Olhar o dado                  | EDA + boilerplate medido, política de localização, vagas PT-BR — 5 defeitos listados + corte de boilerplate com teste |
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

## Fase 1 — Coleta de verdade ✅ fechada 2026-10-18

**Objetivo de aprendizado:** sair de uma fonte para N fontes com formatos de API
diferentes, atrás de uma abstração que **nasce da dor da duplicação** (não antes dela),
e endurecer o coletor para rodar sozinho por 7 dias: timeout, `context`, falha visível,
limiar de erro, rate limit e agendamento.

**Pronto quando:** o coletor roda sozinho por 7 dias, coletando de pelo menos três fontes
atrás da interface `Source`, com dedupe, rate limit por fonte e toda execução medida em `runs`.

### Tarefa 1.1 — Segunda fonte e a interface `Source` ✅ (fechada 2026-10-03)

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

### Tarefa 1.3 — Rodar sozinho sem mentir ✅ (fechada 2026-10-05)

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
- [x] **1.3.4** A run fecha com a verdade: `finished_at` (D12, parte 2) e panic não grava sucesso (D11).
      ✅ 2026-10-05 — migration `006` + `closeRun` grava `finished_at`; `recover` no `defer` (depois
      do `defer` do `closeRun`). `TestPanic` com `Source` falsa; validado por 3 mutações.

### Tarefa 1.4 — Educado com as fontes ✅ (fechada 2026-10-07)

**Objetivo:** falha transitória da fonte (429/5xx) é tentada de novo com espera crescente; o
Himalayas traz todas as páginas; nenhuma fonte recebe requisições mais rápido que o limite dela.

**Critério de aceite:** um servidor de teste que responde 429/5xx é tentado de novo com espera
crescente e desistência no máximo configurado; a coleta do Himalayas passa de 20 vagas; um teste
mede o intervalo entre requisições e ele nunca fica abaixo do configurado no `config.yaml`.

**Conceitos novos:** retry com backoff exponencial, `select` com timer e `ctx.Done()`, contador
atômico em teste concorrente, paginação por cursor, rate limit por intervalo.

**Onde ler:** [Go tour — Select](https://go.dev/tour/concurrency/5) ·
[time.NewTimer](https://pkg.go.dev/time#NewTimer) · [sync/atomic](https://pkg.go.dev/sync/atomic)

**Reordenada em 2026-10-05:** cada fonte faz 1 requisição por run, então rate limit agora seria
abstração antes da dor. Retry primeiro (tem valor já); paginação do Himalayas cria o caso real
de N requisições à mesma fonte; rate limit vem depois dela.

- [x] **1.4.1** Retry com backoff para 429/5xx no client, respeitando o `ctx`; teste unitário em
      `internal/client` (sem banco). ✅ 2026-10-06 — laço de tentativas com `switch` por status;
      espera dobrando via `select` entre `time.NewTimer` e `ctx.Done()`; sem espera depois da
      última tentativa. `run` recebe espera/tentativas (testes do `main` usam 10ms). 4 testes em
      `internal/client`; 5 mutações (sem retry, sem dobrar, tentativa a mais, retry em 404, espera
      ignorando o `ctx`) — todas pegas.
- [x] **1.4.2** Paginação do Himalayas (cursor), com teto de páginas. ✅ 2026-10-07 — `Fetch` em laço
      sobre `range` do teto, cursor via `net/url`, para no cursor vazio; falha em qualquer página →
      erro com o número da página e nenhum item. Teto 5 no `main`. `TestCursor`/`TestHighCeil`/
      `TestMiddleErr` com servidor-roteiro por cursor; 3 mutações pegas. Run #52: `fetched=100`, 2.5s.
- [x] **1.4.3** Timeout, tentativas e espera inicial por fonte no `config.yaml`, com o `Load`
      recusando valor ausente/zero em fonte ligada (D38). ✅ 2026-10-07 — `timeout` virou o
      **orçamento da fonte** (antigo `CollectionTimeOut`); timeout HTTP segue 30s fixo; teto do
      Himalayas segue no código. `Load` valida fontes ligadas (`<= 0` → erro com fonte e campo);
      `TestLoad` com 3 casos e 3 mutações pegas.
- [x] **1.4.4** Rate limit por fonte (intervalo mínimo entre requisições, no `config.yaml`); teste
      mede o intervalo. ✅ 2026-10-07 — no client (um por fonte): guarda o fim da última requisição
      e, **dentro** do laço de tentativas, espera só o que falta (`time.Until`), com `select` e
      `ctx.Done()`. `interval` no `config.yaml` (1s), validado no `Load`. `TestInterval` (3 chamadas,
      intervalos ≥ 50ms) e `TestIntervalCtx` (corte pelo `ctx` em < 500ms); 4 mutações pegas.
      Run #55: Himalayas 6.3s (antes 2.5s).

**Dividida em 2026-10-07:** a antiga 1.4.3 (rate limit + config) passava de 90 min. Config primeiro,
para o intervalo do rate limit já nascer no `config.yaml`.

**Reordenada em 2026-10-07: 1.6 → 1.5 → 1.7.** Medido em 2026-10-07: **0** vagas repetidas entre
fontes em 607 (empresa + título, sem caixa nem pontuação); as 6 empresas presentes em 2+ fontes têm
vagas diferentes. Dedupe agora seria abstração antes da dor. A 1.6 vai primeiro porque os 7 dias são
de calendário — o relógio corre enquanto o resto é feito — e a semana de dados alimenta a 1.5.

### Tarefa 1.6 — Agendamento e os 7 dias ✅ fechada 2026-10-16

**Objetivo:** o coletor vira um processo que coleta sozinho a cada N horas, roda no Raspberry Pi 5
(arm64, Debian) dentro do compose e avisa quando uma fonte parece quebrada.

**Critério de aceite:** 7 dias seguidos de runs em `runs` no Pi, no intervalo configurado, sem
nenhuma intervenção manual; uma consulta mostra runs por dia e por fonte; uma fonte com 0 itens ou
erro em K ciclos seguidos gera um aviso no log (D36).

**Conceitos novos:** `time.Ticker`, desligamento gracioso com `signal.NotifyContext`, injetar a função
do ciclo para testar o laço sem banco, `restart` do compose, deploy em arm64, consulta com janela.

**Onde ler:** [signal.NotifyContext](https://pkg.go.dev/os/signal#NotifyContext) ·
[time.Ticker](https://pkg.go.dev/time#Ticker) ·
[Compose `restart`](https://docs.docker.com/reference/compose-file/services/#restart)

- [x] **1.6.1** Laço de coleta: `collect_every` no `config.yaml`, primeira coleta na hora, ciclo
      com falha vira log `ERROR` e o processo segue; SIGINT/SIGTERM encerram; flag `-once` mantém
      a passada única. Teste do laço sem banco. ✅ 2026-10-07 — `collector(ctx, logger, interval,
      cycle)` recebe o ciclo como função (o `main` fecha `run` numa closure); `TestCollector` +
      `TestFirstCycle` com ciclo falso, 4 mutações pegas; ao vivo: runs #56–#61.
- [x] **1.6.2** Parada limpa e compose de daemon (local): o ciclo recebe o `ctx` do sinal e a
      coleta em andamento é cancelada (run fechada com erro); `app` com `restart`, `analyzer` sem
      esperar o `app` terminar, `config.yaml` montado como volume por cima do da imagem (revisita a
      decisão de 2026-09-15), rotação de log. ✅ 2026-10-08 — prazos derivados do `ctx` (o
      `closeRun` fica com `Background`), laço de fontes para no cancelamento; `TestCtxCancel` com 2
      mutações pegas; ao vivo: `stop` em 0.33s (run #70 fechada com `context canceled`) e
      `collect_every: 1m` sem rebuild.
- [x] **1.6.3** Deploy no Pi: Docker no Pi, clone, `.env`, `up -d --build` nativo em arm64.
      Aquecimento: os dados já entram, o relógio ainda não conta. ✅ 2026-10-08 — Pi em `88cdde0`,
      banco novo (runs #1–#3; Himalayas com 20 páginas: 400 vagas em 23.5s). O restart do Docker
      (no lugar do reboot) religou os containers sozinho, mas revelou a largada sem `depends_on`:
      o 1º ciclo falhou com `connection refused` → próximo só em 6h. O critério "volta sozinho e
      coleta" passou para a 1.6.4.
- [x] **1.6.4** Esperar o banco na partida: antes do primeiro ciclo, tentar conectar com backoff
      respeitando o `ctx`, com prazo total; esgotou → sai com 1 e o `restart` sobe de novo. Prova
      no Pi: restart do Docker → 3 runs novas. ✅ 2026-10-08 — `waitForDB` com backoff dobrando a
      partir de 100ms (`client.Sleep` exportado em vez de copiado); prazo de 2 min derivado do `ctx`
      do sinal no `main` (`DeadlineExceeded` → exit 1, cancelamento → saída limpa);
      `TestWaitForDB` (banco de pé / porta sem ninguém), mutação sem retry pega pelo
      `errors.Is(DeadlineExceeded)`. Commits `f3de19f`, `1b15b5c`. Pi: relatado pelo aluno, sem
      saída colada — a prova com saída vem no redeploy da 1.6.6.
- [x] **1.6.5** Saúde por fonte (D36): ao fim de cada ciclo, fonte com 0 itens ou erro nas últimas
      K runs → `WARN` (K = 2: 12h sem dado no ciclo de 6h). Teste de integração. ✅ 2026-10-08 —
      `sourceLooksBroken` (`store.go`): as K runs mais recentes por `id`, cada uma falha com erro
      **ou** 0 itens, quebrada = K lidas e K falhas; menos de K = fonte nova, não quebrada. No
      `run`, a checagem roda **antes** do `if !success` (o aviso sai justo no ciclo em que tudo
      falhou). `TestSourceLooksBroken` com 5 casos. Ao vivo (cópia, remotive em `.invalid`): 1ª
      passada sem aviso, 2ª com `WARN source is broken source=remotive after=2`.
- [x] **1.6.6** Os 7 dias: redeploy com a 1.6.5 e o relógio começa; consulta de runs por dia e por
      fonte; conferência no 7º dia. ✅ 2026-10-16 — relógio de 2026-10-08 17:56 (run #1 do banco
      do Pi) a 2026-10-16: as 3 fontes com runs **todos os dias**, maior intervalo **6h03**, sem
      intervenção. `db/queries/health.sql` (`de94bb3`). Três quedas de energia no meio (14/10
      ~16:12 e ~16:19, 15/10 14:30): o compose voltou sozinho e o banco foi esperado (prova real da
      1.6.4), mas o 1º ciclo de cada volta falhou por DNS (`lookup … on 127.0.0.11:53: server
      misbehaving` — o roteador volta depois do Pi) → D39. Conferência feita pelo mentor via SSH,
      só leitura.

**Dividida em 2026-10-07:** a antiga 1.6.2 (compose + Pi) passava de 90 min com a parada limpa que
a 1.6.1 revelou (Ctrl+C no meio de uma coleta só valia no fim dela; `docker stop` mata em 10s) e com
o estado do Docker no Pi desconhecido. Local primeiro; o Pi recebe um compose já provado.

**Inserida em 2026-10-08:** a 1.6.4 (esperar o banco) nasceu do aquecimento no Pi — no boot o Docker
religa os containers sem a ordem do `depends_on`, e um 1º ciclo perdido custa 6h, o que quebraria o
critério dos 7 dias. Saúde e 7 dias viraram 1.6.5 e 1.6.6.

### Tarefa 1.5 — Deduplicação ✅ fechada 2026-10-18

**Objetivo:** a mesma vaga aparece uma vez só para quem lê `jobs`: as cópias apontam para a
original por `duplicate_of`.

**Por quê:** medido em 2026-10-16 no banco do Pi copiado (12.584 vagas, 7 dias): empresa + título
sem caixa nem pontuação repetem em **382 grupos dentro da mesma fonte** (1.102 vagas, quase tudo
Himalayas; 190 grupos com descrição idêntica — a CapsLock publicou a mesma vaga 12× no mesmo
segundo, só o número no fim da URL muda) e em **2 grupos entre fontes** (Huzzle, Lemon.io). Sem
dedupe, o ranking e o digest da Fase 5 mostram a mesma vaga N vezes. Em 2026-10-07 (607 vagas)
eram 0 — a dor apareceu com volume.

**Critério de aceite:** entre as vagas vivas (`duplicate_of IS NULL`), nenhum par que a regra da
1.5.2 chama de duplicata — nenhum grupo de chave com mais de uma fonte, e nenhum grupo da mesma
fonte com descrição repetida (grupos como o da Bjak, mesma fonte e descrições diferentes, continuam
com várias vagas vivas de propósito); teste de integração com os casos reais.

**Onde ler:** [`regexp_replace`](https://www.postgresql.org/docs/16/functions-matching.html#FUNCTIONS-POSIX-REGEXP) ·
[agregados e `HAVING`](https://www.postgresql.org/docs/16/tutorial-agg.html)

- [x] **1.5.1** Medir: `db/queries/duplicates.sql` — grupos por empresa + título normalizados,
      entre fontes × dentro da mesma fonte, com quantos têm descrição idêntica; lista dos casos
      entre fontes. ✅ 2026-10-16 — `cross_source` 2/4/0, `same_source` 382/1102/190; 2ª consulta
      com as chaves num `WITH` e volta a `jobs` pelo `JOIN` normalizado (Huzzle, Lemon.io).
- [x] **1.5.2** A regra, em Go, com teste em tabela dos casos reais: chave = empresa + título
      normalizados (minúsculas, só `a-z0-9`); duplicata = mesma chave **e** (fonte diferente **ou**
      descrição idêntica). Os 192 grupos da mesma fonte com descrição diferente **não** são
      duplicata: na Bjak, "Engineering Manager" aparece em duas versões que só diferem no bloco
      Location — uma híbrida, outra remota global. Chave com parte vazia **nunca** é duplicata: 13
      títulos (10 da Gate, em chinês) viram `""` e cairiam todos no mesmo grupo. ✅ 2026-10-17 —
      `dedup.go` (`normalize`, `isDuplicate`) + `TestIsDuplicate` com 7 casos reais (o 7º, MWDN,
      pega a falta do `ToLower`); 5 mutações, cada uma derruba o seu caso.
- [x] **1.5.3** Marcar na gravação: migração `007` com as colunas de chave (`company_key`,
      `title_key`, anuláveis) e índice nas duas; `insertJob` grava as chaves e devolve o `id`; logo
      depois, a vaga aponta `duplicate_of` para a vaga **viva mais antiga que ela** (menor `id`) que
      o `isDuplicate` aceita, ou `NULL` se nenhuma. Teste de integração passando pelo
      `collectAndStoreData` com uma fonte falsa: 3 clones CapsLock (2ª e 3ª → 1ª, sem corrente),
      Bjak (as duas vivas), Huzzle entre fontes, Gate (as duas vivas). ✅ 2026-10-17 — migração
      `007`, `insertJob` com chaves + `RETURNING id`, `updateDuplicate` (um `UPDATE` só, ponteiro
      `nil` = `NULL`), `TestMarkDuplicates` com 10 vagas; mutações `id <= $3` (toda vaga aponta
      para si) e `ORDER BY id DESC` (r2 → h5) derrubam o teste.
- [x] **1.5.4** Histórico, automático: no começo de cada `run`, depois da migração e **antes** das
      fontes, as vagas com `company_key IS NULL` são processadas em lotes, em ordem de `id`: marca
      (`updateDuplicate`) e grava as chaves. O lote é "as próximas N com chave `NULL`": a vaga
      processada sai do filtro sozinha. Teste reaproveita o cenário da 1.5.3 (grava, zera chaves e
      marcas, roda o histórico com lote pequeno). Depois do 1º ciclo no Pi, vira uma consulta
      vazia. **Alavanca do D40:** `UPDATE jobs SET company_key = NULL` faz o próximo ciclo
      reavaliar tudo, em ordem. ✅ 2026-10-18 — `backfillDedupe` (lotes de 1000, prazo de 5 min,
      devolve quantas terminou também no erro), log com `duration`; `TestBackfillDedupe` com lote
      3 (4 lotes, 10 vagas, 2º passe = 0). Mutações: um lote só, `ORDER BY id DESC` e sem marcar —
      as três derrubam o teste. Na cópia local do Pi (transação desfeita): 12.584 vagas em 2,7 s,
      448 marcadas (446 mesma fonte, 2 entre fontes), 0 corrente, critério de aceite 0 / 0.
- [x] **1.5.5** Deploy no Pi e re-medir lá (consulta de duplicatas só nas vagas vivas): o critério
      de aceite. ✅ 2026-10-18 — Pi em `26cccd7`: migração `007` em 87 ms, histórico de **15.279
      vagas em 1 min 20 s**, 565 marcadas. `db/queries/live_duplicates.sql` (chaves gravadas, só
      vivas, chaves não vazias): **`0 | 0`**; sem o filtro de vivas, `3 | 292` — a consulta enxerga
      os grupos.

### Tarefa 1.7 — Fechamento da Fase 1 ✅ fechada 2026-10-18

**Objetivo:** os débitos que a Fase 1 deixou no coletor pagos, e um README que leva um clone até
a coleta rodando.

**Por quê:** a Fase 2 lê o banco que o coletor enche; ela começa sobre um coletor que não perde
ciclo à toa nem entra em panic por resposta de rede, e com o caminho de instalação escrito.

**Pronto quando:** D37 e D39 pagos com teste; D17 fechado; o README descreve as 3 fontes, o
`config.yaml`, o laço de coleta, a dedupe e o deploy no Pi, e um clone limpo segue ele até a
primeira coleta.

**Onde ler:** [expressões de slice](https://go.dev/ref/spec#Slice_expressions) ·
[`time.Timer`](https://pkg.go.dev/time#Timer)

- [x] **1.7.1** D37: a Remote OK com resposta `[]` vira erro da fonte com mensagem própria, não
      `panic` recuperado com "slice bounds out of range". Array só com o aviso legal = feed vazio,
      zero itens sem erro. Teste em tabela com servidor `httptest` (`[]`, só o aviso, aviso + 2
      vagas). ✅ 2026-10-18 — `TestRemoteOK` do aluno; conserto pelo mentor, a pedido
      (`remoteok: empty feed, the legal notice is missing`). Mutações: sem a conferência → `panic`
      em `remoteok.go`; `[]` sem erro → `expected err got nil`.
- [x] **1.7.2** D39: ciclo que falha inteiro tenta de novo em poucos minutos, com teto de
      tentativas, antes de voltar ao ritmo do `collect_every`. Teste no `collector` com ciclo
      falso e intervalos curtos. ✅ 2026-10-18 — `runWithRetries` (3 novas tentativas, 5 min
      entre elas, `client.Sleep` com o `ctx`), usado na partida e em cada tick; `collector`
      recebe as duas como parâmetro. `TestCollectorRetry` (recupera, desiste no teto, cancelado na
      espera) e `TestFirstCycle` com espera de 1s. Cinco mutações, todas derrubam.
- [x] **1.7.3** README da Fase 1 + D17 (`Makefile` lendo a DSN do `.env`, boilerplate do mentor —
      feito em 2026-10-18: `make gstatus` lê o `.env`). ✅ 2026-10-18 — README escrito pelo mentor,
      a pedido: arquitetura com as 3 fontes, o que acontece em cada falha, tabelas, regra da dedupe,
      `config.yaml` campo a campo (montado `:ro`, `restart` basta), `-once`, `make`, consultas de
      `db/queries/`, Raspberry Pi, status. Comandos conferidos contra `compose.yaml`, `config.yaml`,
      `Makefile` e o código; `health.sql` e `make gstatus` rodados. Clone limpo **não** rodado (o
      `container_name` fixo, D23, exige parar o Postgres local).

---

## Fase 2 — Olhar o dado ✅ fechada 2026-10-22

**Objetivo de aprendizado:** explorar dado real com pandas antes de qualquer modelo; transformar
observação em número e número em decisão (o que cortar, o que filtrar); escrever a primeira função
Python "de produção" — type hints, `pytest`, `ruff`, `mypy --strict` — a partir do que a
exploração mostrou.

**Pronto quando:** `docs/eda.md` lista **5 defeitos concretos** do dado (cada um com número,
exemplo e dono) e as políticas de **localização** e de **vagas PT-BR**, cada uma medida; a função
de **corte de boilerplate** existe no `analyzer` com `pytest` de casos reais; `ruff`, `mypy
--strict` e `pytest` limpos.

**Ponto de partida (medido no Pi em 2026-10-18):** 15.279 vagas, 14.714 vivas. **99% Himalayas**
(15.123); Remote OK 137, Remotive 19 — os dois feeds devolvem só as vagas mais recentes. Descrição
média de ~5,4 mil caracteres, em **HTML em 100%** das da Himalayas, com entidades (`&amp;`…) em
metade; `posted_at` vazio em **todas** (D33).

**Onde ler:** [pandas — primeiros passos](https://pandas.pydata.org/docs/getting_started/index.html) ·
[psycopg — uso básico](https://www.psycopg.org/psycopg3/docs/basic/usage.html) ·
[uv — grupos de dependência](https://docs.astral.sh/uv/concepts/projects/dependencies/#dependency-groups) ·
[pytest](https://docs.pytest.org/en/stable/getting-started.html)

### Tarefa 2.1 — Bancada de EDA ✅ fechada 2026-10-19

**Objetivo:** um script de exploração no `analyzer` carrega as vagas vivas, com dado fresco do Pi,
num `DataFrame`, e mostra o primeiro retrato delas.

**Por quê:** toda a Fase 2 acontece em cima desse `DataFrame`; o banco local é a cópia de 16/10,
sem a migração `007` nem as marcas de duplicata.

- [x] **2.1.1** Dado fresco do Pi no banco local (boilerplate do mentor: `pg_dump` pelo `ssh`,
      `dropdb`/`createdb`/`pg_restore`); grupo de dependências `eda` no `analyzer` (`pandas`,
      `ipython`), fora do `dev` e da imagem; `analyzer/eda/01_overview.py`, rodado com `%run` no
      IPython, carrega as vagas vivas com o nome da fonte: `shape`, `dtypes`, contagem por fonte.
      (Revisto em 2026-10-19: o aluno explora num REPL IPython, não no Jupyter.)
- [x] **2.1.2** Primeiro retrato, no mesmo `01_overview.py`: tamanho das descrições por fonte
      (`describe`, histograma), empresas que mais publicam, vagas novas por dia (primeiro
      `fetched_at` em `raw_jobs`, já que `posted_at` está vazio).

### Tarefa 2.2 — Defeitos do dado ✅ fechada 2026-10-20

**Objetivo:** `docs/eda.md` lista 5 defeitos concretos do dado, cada um com número, exemplo e dono.

**Por quê:** cada defeito vira uma decisão (consertar no Go, no Python ou aceitar) antes de o
extrator da 3a ler o texto.

- [x] **2.2.1** Defeitos de texto: a carga (SQL → `DataFrame`) vira o módulo `eda/db.py`, usado
      pelo `01_overview.py`; `eda/02_text_defects.py` mede por fonte a fração de descrições com
      HTML, de títulos com entidade HTML e de descrições com mojibake (D10), com um exemplo de cada.
- [x] **2.2.2** Campos descartados e sentinelas: no `payload` de `raw_jobs`, que campos de data,
      localização, salário e tags cada fonte traz e quanto vêm preenchidos, contra o que chega em
      `jobs` (D33); sentinelas como `salary_min: 0` (D10).
- [x] **2.2.3** `docs/eda.md` com os 5 defeitos (número, exemplo, dono), escolhidos entre os da
      2.2.1, da 2.2.2 e o teto de páginas da Himalayas (D42). (Feita com os 7 candidatos, a pedido
      do aluno.)

### Tarefa 2.3 — Boilerplate ✅ fechada 2026-10-21

**Objetivo:** uma função do `analyzer` corta das descrições os parágrafos que se repetem entre
vagas (boilerplate), com o limiar que a medição sugerir.

**Por quê:** o texto que chega ao extrator da 3a e ao Jev (invariante) precisa ser o da vaga, não
o "sobre a empresa" repetido em centenas de vagas.

**Onde ler:** [`html.parser`](https://docs.python.org/3/library/html.parser.html) ·
[pytest — parametrize](https://docs.pytest.org/en/stable/how-to/parametrize.html) ·
[mypy — `--strict`](https://mypy.readthedocs.io/en/stable/command_line.html#cmdoption-mypy-strict)

- [x] **2.3.1** HTML → parágrafos: `analyzer/text.py` transforma o HTML de uma descrição na lista
      dos seus parágrafos de texto, sobre `html.parser.HTMLParser` (stdlib); `pytest` no grupo
      `dev`, table-driven, com casos pequenos e trechos reais das três fontes; `ruff` e
      `mypy --strict` limpos.
- [x] **2.3.2** Parágrafos repetidos: `eda/04_boilerplate.py` aplica a função às vivas e mede
      quantos parágrafos se repetem, por empresa e no geral, e que fração do texto eles ocupam.
- [x] **2.3.3** Corte de boilerplate com o limiar da 2.3.2, testado com casos reais; mede quanto
      texto sai por fonte. Paga o D43 (caracteres invisíveis) antes.

### Tarefa 2.4 — Localização e PT-BR ✅ fechada 2026-10-22

**Objetivo:** `docs/eda.md` traz duas políticas medidas: quais vagas servem a quem mora no Brasil
(aceita / rejeita / dúvida) e quais estão em português (lote separado, invariante).

**Por quê:** o ranking da 3a e o digest da 5 só fazem sentido sobre vagas que você pode aceitar, e
a Fase 4b precisa saber quantas vagas vão para o lote PT-BR.

**Onde ler:** [pandas — `crosstab`](https://pandas.pydata.org/docs/reference/api/pandas.crosstab.html) ·
[`re` — expressões regulares](https://docs.python.org/3/library/re.html)

- [x] **2.4.1** Localização: `eda/05_location.py` classifica cada vaga viva pelo campo de
      localização da sua fonte (lista de países na Himalayas, texto livre no Remote OK e na
      Remotive) em aceita / rejeita / dúvida para quem mora no Brasil, e conta por fonte.
      ✅ 2026-10-22 — escrita pelo mentor, a pedido do aluno (insistência, §0). Uma função por
      fonte num dict; Himalayas usa país **ou** fuso `-3`, Remotive casa tokens inteiros contra
      um set que inclui o Brasil, Remote OK vira `unknown` em bloco (campo é cidade de quem
      postou). Dado: 20.029 vivas → 1.173 aceita / 18.716 rejeita / 140 dúvida (100% cobertas).
      Achado: na Himalayas o sinal de fuso **subsume** o de país (país ⊂ fuso: 406 ⊂ 1.162).
- [x] **2.4.2** Idioma: `eda/06_language.py` estima o idioma de cada descrição por palavras
      frequentes de cada língua (à mão, sem biblioteca) e conta as vagas em português por fonte.
      ✅ 2026-10-22 — escrita pelo mentor, a pedido (insistência, §0). Contagem PT/EN por set de
      palavras de função. Achado que virou a hipótese: PT é 2,8% no total mas **18% dentro do
      `accept`** (214/1.173) → o lote separado se mantém. Limitações viram **D45**: o detector
      confunde PT/ES (das 214, ~92 PT / ~100 ES) e ~20 vagas PT-BR caem em `reject` por
      `locationRestrictions` errado (falso-negativo localização × idioma).
- [x] **2.4.3** Políticas no `docs/eda.md`: as duas, com número, exemplo e decisão.
      ✅ 2026-10-22 — escrita pelo mentor, a pedido. Seção `# Policies` no `docs/eda.md`
      (localização + idioma), com os números do dump de 22/10 (20.029 vivas), exemplos, decisão
      e referências a D33/D10/D45. **Tarefa 2.4 fechada.**

### Tarefa 2.5 — Fechamento da Fase 2 ✅ fechada 2026-10-22

Decisão tomada em conjunto (2026-10-22): **todos os débitos reagendados**, consumer-driven —
nada pago especulativamente, porque a EDA em Python gerou trabalho de Go sem consumidor ainda.
D10/D32/D33 → **3a** (extração); D42 → **backlog** (revisitar se a cobertura limitar a 3a);
D41 → **3a/5** (quando o analyzer virar serviço real); D44 → **5** (digest); D45 → **4b**.
README da parte Python escrito (`analyzer/README.md`, que também preenche o `readme` do
`pyproject`). Donos atualizados no catálogo do `PROGRESS.md` e no `docs/eda.md`. **Fase 2 fechada.**

---

## Fase 3a — Extração no código ✅ (fechada 2026-10-26)

**Objetivo de aprendizado:** escrever extração de verdade em Python (regex, dicionário, parsing
de texto estruturado), fazer Go e Python dividirem a responsabilidade de "dado limpo" (Go
normaliza e preenche colunas; Python extrai significado), e produzir o primeiro ranking —
TF-IDF — que casa currículo × vaga. É a fase que transforma texto em números que o digest da
Fase 5 vai usar.

**Pronto quando:** o extrator lê o `cv.md`, acha as skills dele, e elas batem com uma lista
conferida à mão (critério da fase); o `analyzer` extrai skills, salário, anos exigidos e idade
das vagas vivas e grava (`job_skills`, colunas de `jobs`); e um ranking TF-IDF ordena as vagas
contra o `cv.md` e grava em `scores` com `model_version`, com as skills do currículo aparecendo
no topo.

**Herda da 2.5 (débitos, consumer-driven):** D10 (entidades HTML cruas + mojibake no texto),
D32 (normalização deixou de ser idempotente sobre `raw_jobs` → `raw_job` órfão + reprocessamento
perdido), D33 (`posted_at`/salário/`location`/`tags` mapeados mas não gravados). Os três são o
conteúdo da Tarefa 3a.3, não um imprevisto.

**Ponto de partida:** `jobs` já tem as colunas `salary_min`/`salary_max`/`currency`/`posted_at`
(vazias — D33); `job_skills (job_id, skill, confidence)` e `scores (job_id, model_version,
score)` prontas e sem uso. A Fase 2 deixou `analyzer/text.py` (HTML → parágrafos) e
`analyzer/boilerplate.py` (corte de repetição) — o pipeline de texto que o extrator consome.

**Onde ler:** [`re` — expressões regulares](https://docs.python.org/3/library/re.html) ·
[SELECT … FOR UPDATE / SKIP LOCKED](https://www.postgresql.org/docs/16/sql-select.html#SQL-FOR-UPDATE-SHARE) ·
[scikit-learn — TfidfVectorizer](https://scikit-learn.org/stable/modules/generated/sklearn.feature_extraction.text.TfidfVectorizer.html)

### Tarefa 3a.1 — `cv.md` e o extrator de skills (Python, sem banco)

**Objetivo:** o caminho mais fino da fase — um extrator que lê um texto e devolve as skills de um
dicionário que aparecem nele, provado primeiro contra o `cv.md`. Sem banco, sem Go: chegar ao
verde e ganhar tração.

**Critério de aceite:** rodar o extrator sobre o `cv.md` imprime a lista de skills encontradas, e
ela bate com o que você lê no arquivo a olho; `pytest`, `ruff` e `mypy --strict` limpos.

**Conceitos novos:** fronteira de palavra em regex (`\b`), `re.escape`, `re.IGNORECASE`, ler
arquivo com `pathlib`, o dicionário como estrutura de dados.

**Onde ler:** [`re` — syntax](https://docs.python.org/3/library/re.html#regular-expression-syntax) ·
[`pathlib.Path.read_text`](https://docs.python.org/3/library/pathlib.html#pathlib.Path.read_text)

- [x] **3a.1.1** `cv.md` (seu currículo real, em markdown) + um dicionário semente de skills + a
      função que acha no texto as skills do dicionário, respeitando fronteira de palavra; `pytest`
      com casos reais (incluindo os nomes difíceis: `Go`, `C++`, `R`). Roda sobre o `cv.md` e
      imprime. ✅ 2026-10-22 — `analyzer/skills_extractor.py` (função + `SKILLS` + `__main__` que
      roda sobre o `cv.md`), 8 testes. Abordagem do aluno (`re.escape` + fronteira + `IGNORECASE`);
      dicionário/impressão, `__main__` e o conserto da pegadinha `C++` feitos pelo mentor a
      pedido/insistência. Pegadinha: `\bC++\b` não casa `C++` (o `\b` final exige transição
      palavra↔não-palavra, e `+` já é não-palavra) → trocado por `(?<!\w)…(?!\w)`, que só exige o
      vizinho externo fora de palavra. Guarda `Go`/`Google` no teste contra regressão.
- [x] **3a.1.2** Sinônimos e confiança: skills com aliases (`JS`/`JavaScript`, `Postgres`/
      `PostgreSQL`) mapeiam para um nome canônico; a função devolve também uma confiança por skill.
      ✅ 2026-10-23 — `skills.py`: `ALIASES` (forma escrita → canônico, universo =
      `set(ALIASES.values())`, sem lista paralela), `skills_extractor(text, skills) -> dict[str,
      float]` soma a contagem no canônico via `.items()`. 7 testes. `ALIASES` e migração dos testes
      feitos pelo mentor a pedido; o aluno resolveu o lookup case-insensitive somando no valor `v`
      (melhor que normalizar caixa) e a iteração `.keys()` → `.items()`. Pendência levada à 3a.2:
      confiança é contagem crua, não normalizada. **Tarefa 3a.1 fechada.**

### Tarefa 3a.2 — O extrator nas vagas (entra o banco)

O mesmo extrator roda sobre as vagas vivas: lê `jobs`, passa a descrição pelo pipeline de texto da
Fase 2 (`text.py` + `boilerplate.py`), extrai as skills e grava em `job_skills`. Entra a fila de
trabalho por índice parcial (`scored_at IS NULL`) e a resposta do Postgres para duas instâncias
(`FOR UPDATE SKIP LOCKED`, SPEC §3).

- [x] **3a.2.1** O pipeline sobre texto de vaga real: carregar as vagas vivas, passar a descrição
      por `text.py` → `boilerplate.py` → `skills_extractor`, e imprimir a distribuição de skills
      pelas vagas. Sem gravar ainda — prova que o extrator sobrevive ao HTML sujo de verdade (D10),
      não só ao `cv.md` limpo. ✅ 2026-10-23 — `eda/07_skills.py` (dois-passos: coleta + `counter`
      do corpus, depois corta e extrai; presença por skill com `Counter.update(found.keys())`).
      Distribuição sobre 20.029 vivas: Go 2665, Python 2272, SQL 1923, CI/CD 1524, …, C++ 282.
      **Achado:** D10 **não deforma** a extração de skills (Remote OK é 137/20.029, e mojibake/
      entidades estragam não-ASCII, não os tokens ASCII das skills) → o que puxa a 3a.3 passa a ser
      o D33, não a limpeza de texto. Limpeza (nomes/prints/anotações) feita pelo mentor a pedido.
- [x] **3a.2.2** Gravar em `job_skills` `(job_id, skill, confidence)`, idempotente, para as vagas
      pendentes. ✅ 2026-10-23 — `main.py`: `skills_per_job(rows)` (sem pandas em produção) +
      `insert_job_skills(conn)` (cursor, `%s`, `executemany`, `ON CONFLICT (job_id, skill) DO
      UPDATE`), chamado pelo `main` dentro de um `with conn:`; erro de banco sobe e vira exit 1.
      13.296 pares em 6.434 vagas; rodar 2× mantém a contagem. Caminho: aluno; `ALIASES` expandido
      (12 → 103 formas: linguagens, frontend, frameworks, bancos, cloud/devops, dados/ML, tools) a
      pedido. **Limites conhecidos:** `ON CONFLICT DO UPDATE` não remove skill que sumiu quando a
      descrição muda (idempotente só no caminho feliz); confiança é contagem crua; matcher não faz
      longest-match (C/R de fora; `Shell` casa em "Bash shell"). Normalização → 3a.5.
- [x] **3a.2.3** Concorrência: `SELECT … FOR UPDATE SKIP LOCKED` para duas instâncias do analyzer
      não processarem a mesma vaga (SPEC §3). ✅ 2026-10-23 — migração `008` (`skills_extracted_at`,
      marcador de fila próprio, separado do `scored_at` do score); laço claim→processa→marca→commit
      por lote (`with conn.transaction()`), Fase A (counter do corpus) uma vez antes. Drenou 20.029
      vivas (4m46), re-run no-op (16s). Pegadinha resolvida pelo mentor a pedido: leitura da Fase A
      com `autocommit=False` abria transação implícita → cada `with conn.transaction()` virava
      savepoint (travas presas até o fim); `conn.autocommit = True` torna os commits por lote reais.
      Typos/params (`timestamptz`, `(SKILLS_LIMIT,)`, `(job_id,)`, import de `Counter`) a pedido.
      **Tarefa 3a.2 fechada.**

### Tarefa 3a.3 — Normalização idempotente no Go (D32 · D33)

Varredura de `raw_jobs` → `jobs` que se auto-cura e reprocessa o histórico (D32), preenche
`posted_at` e salário a partir do payload (D33). Reprocessar não exige recoletar (SPEC §4.1).
D10 saiu da mira: a 3a.2.1 mostrou que entidades/mojibake não afetam a extração de skills (texto
não-ASCII vs tokens ASCII); volta se o Jev (4b) precisar do texto limpo.

- [x] **3a.3.1** `insertJob` grava `posted_at` a partir do `Date` que as fontes já mapeiam
      (forward-only). Zero-value de `time.Time` (fonte sem data) → `NULL`, não ano 0001.
      ✅ 2026-10-23 — `insertJob` com `posted_at` no INSERT e no `DO UPDATE`; `var date *time.Time;
      if !job.Date.IsZero() { date = &job.Date }` (ponteiro-na-fronteira, `Job.Date` segue
      `time.Time`). `TestNilPostedAt` (2 casos, `*time.Time` no `Scan`, pega mutação); suíte verde.
      Código do aluno; teste reescrito pelo mentor a pedido.
- [x] **3a.3.2** Varredura idempotente `raw_jobs` → `jobs` (D32): reprocessa o histórico (backfill
      de `posted_at` nas 20k existentes), se auto-cura (órfão deixa de ser permanente), SPEC §4.1.
      ✅ 2026-10-23 — `normalizeRawJobs` (store.go): última versão por identidade (`ROW_NUMBER`
      partition) paginada por cursor no `id`; **lê o lote → fecha `rows` → processa** (pgx só deixa
      uma query ativa por conn — `insertJob` no meio do `rows.Next()` dava `conn busy`); re-roda o
      `ToJob` da fonte real via `newSource`. Flag `-renormalize` no `main.go` (one-shot, abre a
      própria conn, `return`). `TestNormalizeRawJobs` (Remote OK real — `jobSource` grava formato de
      `sources.Job` que o reprocessamento via `newSource` não lê; 1970 do Himalayas confirmado).
      Varredura/flag do aluno; conn-busy + teste reescritos pelo mentor a pedido.
- [x] **3a.3.3** Salário (D33): `sources.Job` ganha os campos, cada fonte extrai do payload, e
      `insertJob` grava `salary_min/max/currency` — sentinela `salary_min: 0` vira `NULL` (defeito 5
      do `eda.md`). A varredura da 3a.3.2 backfilla. Normalização de unidade fica pra 3a.4 (Python).
      ✅ 2026-10-23 — `SalaryMin`/`SalaryMax` → `*float64`, `Currency`/`SalaryPeriod` → `*string`;
      helpers `nonZero`/`nonEmpty` mapeiam `0`/`""` → `nil` (→ `NULL`). Remote OK (min/max) e
      Himalayas (min/max/currency/period) extraem; Remotive deixa salário `NULL` (texto livre →
      Python/3a.4). Migração `009` (`salary_period`). `TestRemoteOKSalary` (sentinela + decimal
      `0.59` não truncado). Estrutura do aluno; tipos/sentinela/teste pelo mentor a pedido.
      **Tarefa 3a.3 fechada.**

### Tarefa 3a.4 — Extração estruturada em Python (salário, anos, idade)

Salário em muitas unidades (4 períodos, 25 moedas — defeito 6 do `eda.md`) normalizado para uma
base comparável; anos de experiência exigidos; idade da vaga a partir de `posted_at`.

- [x] **3a.4.1** Idade da vaga: rodar o backfill do Go (`-renormalize`) pra popular `posted_at`/
      salário no banco local, depois medir em Python a idade das vivas (`now - posted_at`, em dias),
      tratando `posted_at` ausente. ✅ 2026-10-24 — backfill rodado (`009` aplicada; `posted_at` em
      **100%** das 20.029 vivas, salário em 7.999). `eda/08_date.py`: subtração vetorizada de coluna
      (`NaT` se propaga), `.dt.days`, histograma. **Achado:** nenhuma data < 2000 → a sentinela 1970
      do Himalayas **não ocorre** neste dado (pubDate sempre válido); débito do patch em
      `himalayas.go` cai. Idades ~2–85 dias.
- [x] **3a.4.2** Salário normalizado: converter `salary_min/max` + `currency` + `salary_period`
      para uma base comparável (defeito 6), consumindo o que o Go gravou na 3a.3.3.
      ✅ 2026-10-24 — `analyzer/salary.py`: `normalize_salary(salary, currency, period) -> float |
      None`; `TO_ANNUAL` (hourly=2080) e `TO_USD` (26 moedas, snapshot estático). `None`
      moeda/período → assume USD/anual (Remote OK); valor ausente **ou** moeda/período desconhecido
      → `None` (não chuta). 10 testes (`test_salary.py`), `ruff`/`mypy --strict` limpos. Função do
      aluno (hourly/tipos/None corrigidos a pedido); `TO_USD` e os testes pelo mentor. Falta só a
      distribuição no `eda/09_salary.py` (opcional).
- [x] **3a.4.3** Anos de experiência exigidos: estimar da descrição (regex), por fonte.
      ✅ 2026-10-25 — `analyzer/experience.py`: `extract_years(text) -> int | None`, regex construída
      por partes (`(\d+)\+?(?:-\d+)?\s+(years|yrs)\s+(?:of\s+)?experience`; range → mínimo via
      `group(1)`). Recebe texto já limpo (quem chama roda `html_to_paragraphs`). 6 testes
      (`test_experience.py`). Regex do aluno (após iterar nos casos reais). Pelo caminho, `ruff`/
      `mypy` pegaram um bug de refactor no `main.py` (`skills_per_job` usando nomes indefinidos) que
      os 43 testes não pegaram — lição: estática antes do `/check`. **Tarefa 3a.4 fechada.**

### Tarefa 3a.5 — Ranking TF-IDF (`cv.md` × vagas)

TF-IDF sobre o corpus de descrições + o `cv.md`, cosseno currículo × vaga, gravado em `scores`
com `model_version`. As vagas que compartilham as skills do currículo sobem no topo.

- [x] **3a.5.1** TF-IDF (sklearn) sobre o corpus (descrições limpas + `cv.md`), cosseno `cv` ×
      cada vaga, imprime o top-N. Sem gravar — validar que as vagas do topo compartilham as skills
      do currículo (critério da fase). ✅ 2026-10-26 — `eda/11_similarity.py`: pipeline de limpeza +
      `TfidfVectorizer(stop_words="english").fit_transform([cv, *jobs])`, cosseno `m[0]` × `m[1:]`.
      Top-15 todos DevOps/Platform/SRE/Backend-Python (max sim 0.207); bottom-5 em 0.0 (motorista,
      logística DE, texto ZH). **Critério da Fase 3a atendido:** as skills do CV sobem ao topo.
      Estrutura do aluno; caminho do `cv.md` (ancorado em `__file__`) e prints profissionais pelo
      mentor a pedido. Max sim baixa (~0.2) é normal pro TF-IDF — importa a ordem, não a magnitude.
- [x] **3a.5.2** Persistir em `scores` `(job_id, model_version, score)`, idempotente, com um
      `model_version` (ex. `tfidf-v1`); marca `scored_at`. ✅ 2026-10-26 — `insert_scores`/`scorer`
      no `main.py`: corpus inteiro vetorizado de uma vez (CV = doc 0), cosseno, `executemany` +
      `ON CONFLICT (job_id, model_version) DO UPDATE`, `model_version='tfidf-v1'`, `scored_at=now()`.
      Idempotente (2ª rodada seleciona zero; corpus degrada pra 1 doc sem estourar). `sklearn` virou
      dependência de produção → override de `mypy` no `pyproject.toml` (boilerplate do mentor).
      **Tarefa 3a.5 fechada.**

### Tarefa 3a.6 — Fechamento da Fase 3a ✅ (fechada 2026-10-26)

Débitos abertos na fase, README da parte de extração atualizado.

- [x] Auditoria de débitos (2026-10-26): D32 pago; D33 parcial (salário/`posted_at` pagos,
      `Location`/`Tags` → Fase 5); D10 → Fase 4b; **D46 pago** (skills refeitas quando o texto
      muda — reset do marcador no Go + delete-reinsert no Python, `TestNilSkillsExtractedAt`);
      D47 (confiança crua) e D48 (longest-match) → Fase 4c.
- [x] README atualizado: `analyzer/README.md` (fluxo do `main.py`, helpers, EDA `07`–`11`) e
      `README.md` da raiz (descrição do analyzer + status das Fases 2 e 3a).

---

## Fase 3b — Rotulagem e avaliação ✅ (fechada 2026-10-28)

**Objetivo de aprendizado:** nasce o segundo binário Go (`radar`, a CLI do usuário), e com ele
a verdade de ter o rótulo humano como referência. Você rotula ~100 vagas à mão, congela um
conjunto de teste que nunca mais entra em treino, e mede o TF-IDF da 3a nesse conjunto — a
primeira baseline honesta do projeto.

**Pronto quando:** `radar label` existe e grava em `labels` sem nunca mostrar score de modelo;
~100 vagas rotuladas; um split de teste congelado e versionado; e o TF-IDF medido nele, com
`model_version` e tamanho do conjunto reportados.

**Invariantes que entram em jogo (§2.1):** o `radar label` nunca mostra score enquanto você
rotula; o conjunto congelado nunca entra em treino/ajuste; todo número vem com o `model_version`.

**Estrutura Go (SPEC §7):** é aqui que `cmd/` nasce e, por pressão (não antes), `internal/store`
e `internal/config` são extraídos. O collector segue rodando o tempo todo.

### Tarefa 3b.1 — O binário `radar` nasce

A CLI do usuário ganha vida: um segundo `package main` em `cmd/radar/`, que conecta no mesmo
Postgres e, no fim da tarefa, roda a tela de rotulagem. Começa pelo caminho mais fino —
conectar e contar — pra provar a estrutura de dois binários antes de qualquer interação.

- [x] **3b.1.1** `cmd/radar/` nasce: binário que conecta no Postgres (lendo `DATABASE_URL`) e
      imprime uma contagem real (vagas rotuladas × por rotular). Sem interação ainda. Duplicar as
      poucas linhas de conexão é aceito (SPEC §7: extrai pra `internal/` quando doer).
      ✅ 2026-10-26 — `cmd/radar/main.go` (`package main`): `pgx.Connect` + `Ping`, `signal.NotifyContext`,
      contagem `LEFT JOIN labels … WHERE duplicate_of IS NULL` com `FILTER`. Rodado contra o banco:
      `LABELED 0 / 16204`, bate com o `psql`. Furos corrigidos no caminho (`/check` 2×): `package cmd`
      não vira binário (gerava `ar archive`), `print`→`fmt.Printf`, query com `j.labeled_at`
      inexistente + `JOIN` interno, `DATABASE_URL` no log. `go vet` limpo. Dois `package main` no
      módulo (raiz + `cmd/radar`), collector intocado.
- [x] **3b.1.2** `radar label`: mostra uma vaga por vez (título, empresa, descrição limpa) e
      captura o rótulo (`would_apply` + opcionais), gravando em `labels`. **Nunca** mostra score.
      ✅ 2026-10-27 — `label()` no `cmd/radar/main.go`: `QueryRow` uma vaga por vez (sem `conn busy`),
      prompt via `bufio.Reader`, `INSERT` em `labels`, `scored_at`/score nunca exibidos. Verificado
      contra o banco (2 rótulos de teste, campos corretos, `status` subiu, rótulos apagados depois).
      Roteamento `status`/`label` e esqueleto pelo aluno; os três furos finais (aliasing
      `seniority`/`note`, trim no armazenamento, `ErrNoRows` como fim) escritos pelo mentor após o
      aluno mudar o §0 do `CLAUDE.md` pra permitir na insistência. Nit aberto: Ctrl+D (`io.EOF`) loga
      como erro em vez de sair quieto.
- [x] **3b.1.3** Robustez da sessão de rotulagem: retomar de onde parou, pular vaga, sair limpo;
      não re-mostrar vaga já rotulada.
      ✅ 2026-10-28 — a query do `label()` filtra `WHERE l.labeled_at IS NULL ORDER BY j.id`: cada
      run retoma nas não-rotuladas e nunca re-mostra uma rotulada. Ctrl+D (`io.EOF`) sai com
      `"shutting down"` em cada prompt, em vez de logar como erro (fecha o nit da 3b.1.2).
      `cleanDescription`/`clearScreen` deixam uma vaga por tela. **"Pular vaga" cortado por YAGNI:**
      as 122 foram rotuladas sem precisar pular; se fizer falta, vira subtarefa de ~20 min.
      Commit `3045c1e`.

### Tarefa 3b.2 — Rotular ~100 e congelar o split

- [x] **3b.2.1** Rotular ~100 vagas de verdade (sessão sua, não código). — 122 rótulos.
- [x] **3b.2.2** Congelar o conjunto de teste: split versionado (`random_state` fixo) que nunca
      entra em treino nem ajuste; registrar como foi feito. — `analyzer/freeze_test_set.py` →
      `analyzer/frozen.json` (test-set-v1, seed 7, 37 de 122).

### Tarefa 3b.3 — Medir o TF-IDF no conjunto congelado

- [x] **3b.3.1** Rodar o ranking TF-IDF da 3a só no conjunto congelado e reportar a métrica com
      `model_version` e tamanho — a primeira baseline honesta. — `analyzer/evaluate_baseline.py`:
      `tfidf-v1 ROC AUC=0.5000 n=37 pos=5` (sem poder de ranking no conjunto congelado).

### Tarefa 3b.4 — Fechamento da Fase 3b ✅ (fechada 2026-10-28)

- [x] Débitos abertos na fase + README/docs da rotulagem.
      ✅ 2026-10-28 — a fase não abriu 🔴/🟡; o único nit (Ctrl+D) morreu no commit `3045c1e`.
      `.gitignore` passou a ignorar o binário `cmd/radar/radar` e os `.fuse_hidden*` do mount
      (commit `125fa2d`). README raiz ganhou a seção "Labelling jobs (`radar`)" + Phase 3b no
      status; `analyzer/README.md` ganhou "Evaluation: the frozen test set". Débitos pendentes
      são todos de fases anteriores, já reagendados (D8/D9/D23/D24/D25/D26/D33/D40/D41/D42/D44/D45/D47/D48).

---

## Fase 4a — Embeddings vs baseline 📍 (aberta 2026-10-28)

**Objetivo de aprendizado:** trocar a representação do texto — de TF-IDF (conta palavras) para
embeddings densos de um modelo pré-treinado (`sentence-transformers`) — e descobrir, no mesmo
conjunto congelado, se o significado semântico rankeia as vagas melhor que o TF-IDF. Os vetores
moram no Postgres via `pgvector`.

**Pronto quando:** cada vaga viva tem um embedding no banco (`pgvector`); existe um `model_version`
novo no `scores` derivado do cosseno CV×vaga; e o `evaluate` reporta o ROC AUC dos dois modelos lado a
lado no conjunto congelado — você sabe qual ganha e por quanto.

**Invariantes que entram em jogo (§2.1):** o `scores` continua com PK `(job_id, model_version)` e o
embedding entra com `model_version` **novo** (não sobrescreve `tfidf-v1`); o conjunto congelado só é
lido na avaliação, nunca para escolher modelo nem ajustar nada; todo número vem com `model_version` e
tamanho. O modelo é pré-treinado — não há `fit` sobre o corpus, então não há como vazar rótulo.

**Decisões da fase (recomendação do mentor — confirme ou troque):**

- **Modelo:** `all-MiniLM-L6-v2` (384 dimensões) — pequeno, roda em CPU, o padrão de fato para
  similaridade semântica de texto curto. A dimensão da coluna `pgvector` sai daqui: trocar de modelo
  depois = nova migração.
- **Escopo:** embeddar e pontuar **todas as vagas vivas** (como o `tfidf-v1`), não só as rotuladas —
  o score vira um ranking de verdade; a avaliação lê só o subconjunto congelado.
- **Onde roda:** script separado no `analyzer/`, **fora** do `main.py` sempre-ligado — o
  `sentence-transformers` puxa o `torch` (pesado) e o coletor/Pi não precisam dele. Dep em grupo
  próprio do `uv`.
- **Infra (já feita, boilerplate):** a imagem do Postgres no `compose.yaml` virou `pgvector/pgvector:pg16`
  (mesmo PG 16, volume reaproveitado) para o `CREATE EXTENSION vector` existir.

### Tarefa 4a.1 — pgvector entra no banco

- [x] **4a.1.1** Migração goose que habilita a extensão `vector` e cria onde guardar o embedding de
      cada vaga (uma dimensão fixa, a do modelo), com `down` que desfaz. Idempotente.
      ✅ 2026-10-28 — `db/migrations/010_add_vector_table.sql`: `CREATE EXTENSION IF NOT EXISTS vector`
      + tabela `job_embeddings(job_id→jobs.id, model_version, embeddings vector(384), computed_at,
      PK(job_id, model_version))`, espelhando o `scores`. `Down` na ordem inversa (tabela→extensão),
      reversibilidade conferida em transação revertida. Infra: imagem do Postgres recriada para
      `pgvector/pgvector:pg16` (volume reaproveitado, `vector` 0.8.7). Nullable em `embeddings` e nome
      no plural ficaram como nit (não bloqueante).

### Tarefa 4a.2 — Gerar os embeddings

- [x] **4a.2.1** Script no `analyzer/` que carrega o `all-MiniLM-L6-v2` e transforma um texto num vetor
      de 384 floats (encoder provado isolado, antes de tocar o banco). Dep em grupo próprio do `uv`.
- [x] **4a.2.2** Ligar o encoder ao banco: ler as vagas vivas sem embedding, encodar o texto limpo e
      gravar em `job_embeddings`. Idempotente: rodar de novo não reprocessa nada.

### Tarefa 4a.3 — Pontuar com o cosseno

- [x] **4a.3.1** Calcular o cosseno CV×vaga no `pgvector` (operador `<=>`) e gravar em `scores` com o
      `model_version` novo. Idempotente.

### Tarefa 4a.4 — Avaliar e comparar

- [x] **4a.4.1** Generalizar o `evaluate` para medir qualquer `model_version` no conjunto congelado e
      reportar os dois (TF-IDF × embeddings) lado a lado — qual ganha e por quanto.
      Resultado (n=75, pos=5): `tfidf-v1`=0.5086 × `minilm-l6-v2`=0.4600 — embedding **não** bate o baseline.

### Tarefa 4a.5 — Fechamento da Fase 4a

- [ ] Débitos abertos na fase + README/docs dos embeddings.

---

## Fases seguintes (resumo)

**Fase 4b — Jev como extrator de features.** Question set v1, `jev_answers`, piloto com anotação manual, pesos manuais.

**Fase 4c — sklearn.** Features do Jev + cosseno + features do código; três abordagens comparadas.

**Fase 5 — FastAPI e digest.** Digest semanal no Discord chegando sozinho.

**Fase 6 — Kubernetes.** Imagens multi-stage/multi-arch, minikube.

**Fase 7 — Fechamento.** CI, painel de tendências, README verificado por outra pessoa.
