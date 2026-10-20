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

## 📍 Fase 2 — Olhar o dado

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

### 📍 Tarefa 2.2 — Defeitos do dado

**Objetivo:** `docs/eda.md` lista 5 defeitos concretos do dado, cada um com número, exemplo e dono.

**Por quê:** cada defeito vira uma decisão (consertar no Go, no Python ou aceitar) antes de o
extrator da 3a ler o texto.

- [x] **2.2.1** Defeitos de texto: a carga (SQL → `DataFrame`) vira o módulo `eda/db.py`, usado
      pelo `01_overview.py`; `eda/02_text_defects.py` mede por fonte a fração de descrições com
      HTML, de títulos com entidade HTML e de descrições com mojibake (D10), com um exemplo de cada.
- [ ] **2.2.2** Campos descartados e sentinelas: no `payload` de `raw_jobs`, que campos de data,
      localização, salário e tags cada fonte traz e quanto vêm preenchidos, contra o que chega em
      `jobs` (D33); sentinelas como `salary_min: 0` (D10).
- [ ] **2.2.3** `docs/eda.md` com os 5 defeitos (número, exemplo, dono), escolhidos entre os da
      2.2.1, da 2.2.2 e o teto de páginas da Himalayas (D42).

### Tarefa 2.3 — Boilerplate *(resumo)*

HTML → parágrafos de texto (função + `pytest`); medir parágrafos repetidos entre vagas (por
empresa e no geral) e quanto do texto ocupam; `cut_boilerplate` com o limiar que a medição
sugerir, testado com casos reais.

### Tarefa 2.4 — Localização e PT-BR *(resumo)*

O que cada fonte diz sobre localização, em categorias, e quantas vagas servem a quem mora no
Brasil → política aceita / rejeita / dúvida. Quantas vagas estão em português → política de lote
separado (invariante).

### Tarefa 2.5 — Fechamento da Fase 2 *(resumo)*

Débitos com dono na Fase 2 (D10, D32, D33, D41) pagos ou reagendados; README da parte Python.

---

## Fases seguintes (resumo)

**Fase 3a — Extração no código.** `cv.md`, skills por dicionário/regex, anos exigidos, salário, idade da vaga, ranking TF-IDF.

**Fase 3b — Rotulagem e avaliação.** `radar label` (~100 vagas), conjunto de teste congelado, TF-IDF medido nele.

**Fase 4a — Embeddings.** sentence-transformers + pgvector contra a baseline, no conjunto congelado.

**Fase 4b — Jev como extrator de features.** Question set v1, `jev_answers`, piloto com anotação manual, pesos manuais.

**Fase 4c — sklearn.** Features do Jev + cosseno + features do código; três abordagens comparadas.

**Fase 5 — FastAPI e digest.** Digest semanal no Discord chegando sozinho.

**Fase 6 — Kubernetes.** Imagens multi-stage/multi-arch, minikube.

**Fase 7 — Fechamento.** CI, painel de tendências, README verificado por outra pessoa.
