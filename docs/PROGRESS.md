# PROGRESS — job-radar

> Estado entre sessões. Atualizado a cada `/check` aprovado e a cada `/pausa`.

---

## Onde estou

**1.3.1 fechada em 2026-10-05 (branch `feature/client`).** Orçamento por fonte injetado no `run`,
client com 30s fixos, `closeRun` com contexto próprio; `TestClientTimeout` prova que a fonte lenta
é cortada e que a run grava o erro mesmo assim. 🔴 B5 resolvido, 🟡 D7 pago. Próximo: **1.3.2
(D21)** — o motivo do salto para a 1.4 (criar o client e testar o timeout) foi cumprido na 1.3.1.

Última sessão: **2026-10-05**.

---

## Linha do tempo

### 2026-08-25 — Sessão Zero

Projeto já vinha decidido pelo aluno; a sessão foi usada para fechar decisões em aberto
e escrever `SPEC.md` / `ROADMAP.md` / `PROGRESS.md`.

### 2026-08-26

- ✅ **0.1.1** — `002_add_sources.sql` semeando `remoteok` com `ON CONFLICT DO NOTHING` e
  `down` que apaga só a própria linha. Ciclo `up → up → down → up → down-to 0` verificado
  contra Postgres real, limpo. 🔴 **B2 resolvido.**
- ✅ **0.1.2** — `003_add_job_identity.sql`: `(source_id, external_id)` `NOT NULL` com
  `UNIQUE` composta; FK só para `sources`. Verificado em 4 cenários (sem identidade →
  recusa; vaga repetida → recusa; mesmo `external_id` de outra fonte → aceita; vagas
  distintas da mesma fonte → aceita). 🔴 **B3 resolvido.** Tarefa 0.1 fechada.
- ✅ **0.2.1** — coletor grava em `raw_jobs`. `ON CONFLICT ... DO NOTHING RETURNING id` +
  `errors.Is(pgx.ErrNoRows)` distingue "inseri" de "já existia" em uma ida ao banco.
  Refatorado para `run(logger) error`, com `main()` como único ponto de morte. Verificado:
  99 linhas na 1ª execução, 0 novas na 2ª, +1 quando o `content_hash` muda; roda sem `.env`
  desde que `DATABASE_URL` esteja no ambiente. 🔴 **B1 resolvido**, 🟡 **D1 e D2 pagos.**

### 2026-09-03

- ✅ **0.2.2** — normalização `raw_jobs` → `jobs`. Última versão de cada vaga escolhida com
  `ROW_NUMBER() OVER (PARTITION BY source_id, external_id ORDER BY fetched_at DESC)`;
  gravação com `ON CONFLICT ON CONSTRAINT uq_source_id_external_id DO UPDATE`. A regra de
  invalidação da §4.2 saiu de um `CASE` que compara `EXCLUDED.raw_job_id` com
  `jobs.raw_job_id` — barato porque o `unique (source_id, external_id, content_hash)` da
  `001` já garante que esse id só muda quando o conteúdo muda. Verificado: conteúdo igual
  preserva `scored_at`; conteúdo alterado zera `scored_at` **e** move o ponteiro; 3 rodadas
  seguidas 213 → 213 → 213 com 0 erros e 0 ponteiros desatualizados.
  🔴 **B4 resolvido**, 🟡 **D3 pago.**
- 🔴 **B4 — schema drift.** `003_add_job_identity.sql` no disco dizia `NOT NULL`, mas o banco
  aplicado tinha as colunas nulas: a migration foi editada *depois* de aplicada, e o goose
  nunca relê uma versão já registrada. A `uq_source_id_external_id` estava decorativa
  justamente no banco onde o upsert seria testado. Corrigido com `goose down-to 2` + `goose up`
  (seguro: `jobs` estava vazia). **Regra: migration aplicada é imutável — corrige-se com uma
  nova migration, nunca editando a antiga.**
- 🧠 Aprendido: uma query SQL é uma string opaca para o compilador Go. `go build` e `go vet`
  passaram limpos em 4 defeitos distintos (`EXLUDED`, `comptany`, contagem de placeholders,
  tipo errado no `Scan`) que só apareceram em runtime. Motiva a avaliação de sqlc na Etapa 1
  (ainda **não** decidida).
- ✅ **0.2.3** — toda execução deixa rastro em `runs`. `openRun` grava `started_at` logo
  depois da conexão; um `defer` com **função literal** sobre o **resultado nomeado** de `run`
  chama `closeRun` na saída, aconteça o que acontecer. `source_id` e `error` viajam como
  `*int` / `*string` para que `nil` vire `NULL` — a coluna `source_id` de `runs` é nullable
  justamente para a run que morre antes de saber a fonte. Verificado em 4 cenários:
  sucesso (`fetched=101, failed=0`), API 404 (`error` com prefixo `failed to fetch data:`),
  linha `remoteok` ausente de `sources` (`source_id` NULL, `error` com prefixo
  `failed to get source id:`) e banco rejeitando todo INSERT (`fetched=101, failed=100`).
  **Tarefa 0.2 fechada.**
- 🧠 Três armadilhas de ponteiro/`defer` custaram caro e ficam registradas:
  argumentos de `defer` são avaliados na hora do `defer` (só a closure lê o valor final);
  `*p = v` exige que `p` já aponte para algo, `p = &v` é que faz `p` apontar;
  e ponteiro é passado **por valor** — reatribuir o parâmetro dentro da função não sai dela,
  por isso contagem sai pelo `return`, não por parâmetro de saída.

### 2026-09-09

- **0.3.1 — primeiro teste do projeto.** `TestMain` sobe um Postgres via testcontainers,
  aplica as três migrations com `goose.NewProvider` sobre a `embed.FS` e derruba tudo.
  `go test ./... -count=1` → 10s, sem container órfão, sem tocar no banco de dev.
- **A armadilha do ciclo de vida, três vezes.** Primeiro `defer` dentro do `setupPostgres`
  (o container morria antes de o handle chegar ao chamador), depois `teardown()` chamado
  antes do `m.Run()`. Raiz comum: `TestMain` não tem `t.Cleanup`, e `os.Exit` não roda
  `defer` — a limpeza precisa ser explícita, entre o `m.Run()` e o `os.Exit`.
- **Sombreamento silencioso.** `db, teardown, err := setupPostgres()` compila sem aviso
  (`teardown` e `err` são novos, então o `:=` é válido) e deixa a variável de pacote em
  `nil`. Resolvido com `var` antes + `=` puro — a forma que o compilador consegue proteger.
- **`fs.Sub` foi obrigatório:** o goose faz `fs.Glob(fsys, "*.sql")` na **raiz** da FS, e a
  `embed.FS` tem `db/` na raiz. Sintoma sem isso: `no migrations found`.
- **Testes validados por mutação** (não só por passarem): tabela extra, tabela ausente e
  seed removida — as três detectadas.

- **Transações (D16), 2026-09-09.** Três tentativas até a fronteira certa. (1) `Begin` no
  `run()`, envolvendo `openRun`/`closeRun`: como `*pgx.Conn` é **uma sessão só**, todo
  `conn.Exec` seguinte viaja dentro da transação mesmo sem receber a `tx` — o rollback
  apagava a run que falhou (verificado: `runs = 0`). (2) `err = tx.Rollback(ctx)` num
  `defer` sobre resultado nomeado: o `ErrTxClosed` que o `Rollback` sempre devolve depois
  de um `Commit` bem-sucedido virava o erro de `run()` — coleta perfeita saindo com
  `exit=1`. (3) `insertRawJobs` dentro da transação: uma falha em `jobs` desfazia os 101
  payloads bons.
- **Interface `querier`** (3 métodos, definida no consumidor) para as funções aceitarem
  `*pgx.Conn` **e** `pgx.Tx`. `Begin` fica de fora de propósito — quem abre transação é
  `collectAndStoreData`, que continua recebendo a conexão concreta.
- **Verificação final:** sabotagem (`check` que rejeita toda `jobs`) → `raw_jobs = 100`,
  `jobs = 0`, **uma** linha de erro nomeando a vaga, gravada em `runs.error`. Caminho
  feliz duas vezes → `exit=0`, zero linhas `ERROR`, `jobs = 250 = distintas`.

- **`config.yaml` e múltiplas fontes, 2026-09-09 (adiantado da Etapa 1).** Fontes saíram do
  código para `config.yaml` (`name`/`enabled`/`url`), `run()` recebe `dbURL` e `*Config` por
  parâmetro, e o corpo do laço virou `processSource`. Efeito colateral bom: a 0.3.2 fica
  trivial — o teste monta um `Config` apontando para o `httptest.Server`, sem ler arquivo.
- **Dois erros de configuração silenciosa no caminho.** (1) Tag `yaml:"enable"` contra chave
  `enabled:` no arquivo: campo desconhecido é **ignorado** pelo YAML, então `Enabled` ficou
  `false`, o laço nunca entrou e o programa saiu com `exit=0` tendo coletado nada. (2) `defer
  closeRun` dentro do `for`: `defer` se liga à **função**, não ao bloco — as N runs ficariam
  abertas até o fim de `run()` e fechariam em ordem inversa.
- **Verificado com duas fontes** (a primeira ausente de `sources`): run 21 gravada com
  `source_id` NULL e o erro, run 22 coletando normalmente. Uma fonte que falha não derruba
  as outras — `run()` loga `WARN` e segue.

### 2026-09-14

- **0.3.2 fechada — teste de ponta a ponta sem internet.** `httptest.Server` servindo
  `testdata/remoteok.json`, `Config` montado no teste (sem `Load`), `TRUNCATE ... RESTART
  IDENTITY CASCADE` no início e duas execuções de `run` contra o mesmo estado.
- **Três asserções, três frentes independentes:** contagem absoluta (`raw_jobs`/`jobs` = 100)
  pega coletar/normalizar de menos; igualdade entre as duas execuções pega duplicação; e
  `runs` (2 linhas, nenhuma com `error`) pega a segunda execução morrendo no meio — que é
  invisível para as contagens, porque o rollback deixa o banco idêntico ao da primeira.
- **Quatro mutações, três detectadas.** Escapou a remoção do `ON CONFLICT DO NOTHING` de
  `raw_jobs`: os 100 itens colidem, viram `failed++`, e o coletor termina `exit=0` com
  `error` NULL. É o D12/D21 aparecendo como buraco de teste — fecha com uma asserção de uma
  linha (`SELECT COUNT(*) FROM runs WHERE failed > 0` = 0).
- **Armadilhas do caminho:** `run()` devolve `nil` mesmo com a fonte explodindo (D21), então
  checar o retorno de `run` não basta — a falha só existe em `runs`. E `db.QueryRow` sem
  `.Scan` engole o erro: um `SELECT ... FROM raw_jogs` ficou verde por rodadas.

---

### 2026-09-15

- **0.4.1 fechada — o collector vive dentro do compose.** `Dockerfile` multi-stage
  (`golang:1.26.1-alpine` → `alpine:3.23`), `.dockerignore`, serviço `app` no `compose.yaml`
  construído pelo próprio compose, credenciais fora do arquivo versionado e
  `condition: service_healthy`. Verificado: `docker image rm -f job-searcher:latest` seguido
  de `docker compose up` reconstrói e coleta sozinho; `postgres_db Waiting → Healthy → app
  Starting`; run 32 com `fetched=100, failed=0, error NULL`, `app Exited (0)`.
- **935 MB → 36.9 MB.** O estágio builder (compilador Go + cache de módulos + fonte) pesa
  935 MB; a imagem final carrega dois arquivos, `jobs_collector` e `config.yaml`. `FROM` é
  linha de corte: o estágio final nasce de uma base limpa e só atravessa o que o
  `COPY --from` leva na mão.
- **Quatro nomes que não casavam, em dois dias.** `yaml:"enable"` (09/09), `CGO_ENABLE=0`,
  `job_searcher` no `.dockerignore`, `js_builder` vs `js-builder` no `docker run`. Só o
  último deu erro. Regra que saiu disso: **ferramenta que *resolve* nome protege você;
  ferramenta que *filtra* nome, não** — num filtro, entrada que não casa com nada é
  legítima, então a verificação é sempre olhar o resultado, nunca reler a própria lista.
- **O `CGO_ENABLE=0` era inofensivo por acidente:** `golang:*-alpine` não traz compilador C,
  então o Go já resolvia `CGO_ENABLED=0` sozinho. Trocar o builder para a variante Debian
  teria produzido binário ligado a glibc e um `not found` inexplicável dentro do alpine.
- **`go run .` quebrado e consertado.** Ao apontar a `DATABASE_URL` para a rede do compose,
  o terminal perdeu o acesso. Causa: uma string única respondendo uma pergunta cuja resposta
  depende de quem pergunta. Ver decisão de identidade × topologia abaixo.
- **`service_started` é o `WithOccurrence(1)` do compose.** Mesma corrida que o
  `main_test.go:236` já tinha resolvido com testcontainers: o entrypoint do Postgres sobe um
  servidor temporário para o initdb antes do definitivo. Não é observável hoje porque o
  volume local já está inicializado — só aparece na máquina de quem clona, que é exatamente
  o cenário do critério de aceite da Tarefa 0.4.

### 2026-09-29

- **Sincronização com o `CLAUDE.md` novo:** "Etapa" → "Fase", 3a/3b, 4a/4b/4c, Jev, invariantes
  de avaliação em `SPEC.md` e `ROADMAP.md`.
- ✅ **0.4.2 — o Python enxerga o banco.** `analyzer/` como projeto `uv`, `psycopg[binary]`
  (a variante pura exige a libpq do sistema, ausente na slim), Dockerfile em duas camadas
  (`uv sync --no-install-project` antes do `COPY . .`, mesma ideia do `go mod download`).
  Verificado: `app` sai 0 → `analyzer` sobe e imprime 363 = `psql`.
- **Quatro armadilhas no caminho:**
  - `dockerfile:` é resolvido **a partir do `context`** — `./analyzer/` + `./analyzer/Dockerfile`
    virou `analyzer/analyzer/Dockerfile`.
  - `uv run` sincroniza antes de executar e inclui o grupo `dev` por padrão: o container
    baixava mypy e ruff a cada subida.
  - `load_dotenv()` **sobe diretórios** a partir do script até achar um `.env`. Com ele no
    código, o teste de "variável ausente" era impossível: ele recolocava a variável que o
    teste tinha tirado. É o "filtro × resolvedor" de 15/09 com outro rosto.
  - `if` que imprime e não retorna não é tratamento de erro, é log.
- **`service_completed_successfully` × `service_healthy`:** o `analyzer` depende de o coletor
  ter **terminado**, não de o banco estar de pé.

### 2026-09-30

- ✅ **0.4.3 — máquina limpa.** O coletor migra ao subir: `//go:embed` saiu do `_test.go`
  (que o `go build` ignora) para o `main.go`, junto com o import `_ ".../pgx/v5/stdlib"`, que
  só **registra** o driver `"pgx"` — sem ele, erro em runtime, não em compilação. Log por
  migration aplicada e `goose.PartialError` via `errors.As`, que diz o que entrou e o que
  quebrou. **Tarefa 0.4 e critério da Fase 0 fechados.**
- **O primeiro teste de clone passou pelo motivo errado — e falhou.** O compose reaproveitou a
  `job-searcher:latest` de 29/09, sem o `migrate` (D25). Diagnosticado lendo o **qual** erro:
  `relation "runs" does not exist` no `app` e nenhuma linha de migration. E o `app` saiu 0
  mesmo sem nenhuma tabela — D21 ao vivo, com o `analyzer` subindo em cima.
- **`defer` antes do `if err`, de novo** (`sql.Open`) — mesmo padrão do D15 de 09/09.
- **`.env.example` com `${}`:** a `DATABASE_URL` é montada a partir das outras chaves; a senha
  de exemplo existe uma vez. Porta `5433` (visão do terminal), não `5432` — no laptop, a 5432
  é **outro** Postgres, e o `migrate` criaria o schema lá sem erro nenhum.
- ✅ **0.5.1 — débitos do fim da fase.** D22 feito pelo mentor a pedido (mecânico). D20: com
  `(err error)` na assinatura, `return X` atribui `X` ao resultado **antes** dos `defer` — o
  `runs.error` passou a guardar o erro envolvido. D18: `Debug` por item + uma linha por fonte;
  o coletor saiu de 100+ linhas para 2.
- ✅ **0.5.2 — README** (escrito pelo mentor a pedido). O aluno declarou o teste do leitor feito,
  mas as imagens não tinham sido reconstruídas; o mentor repetiu o teste num clone:
  quick start e `go test` passam. **Fase 0 fechada.**
- **"Funciona na minha máquina", versão permissão de arquivo:** o SSD do projeto marca todo
  arquivo como executável e o repo tem `core.fileMode=false`. O `main.py` tem shebang, o git
  o guarda como `100644`, e num clone em `/tmp` o `ruff` acusa `EXE001`. No diretório de dev,
  passa. Só um clone num filesystem normal mostra — mesmo padrão do D25.

### 2026-10-02 — abertura da Fase 1 (diagnóstico)

- **Diagnóstico de início de sessão sobre o código real:** `go build`/`go vet` limpos,
  `ruff check` no analyzer limpo, árvore de trabalho limpa. **Nenhum 🔴.** (Testes de
  integração não re-rodados — exigem Docker/testcontainers; último verde em 2026-09-30, sem
  mudança de código desde então.)
- ✅ **D27 encontrado já resolvido.** O shebang saiu de `analyzer/main.py` (começa em
  `import os`), modo git `100644`, `ruff` passa em ambiente limpo. Fechado sem trabalho.
- **`testdata/remotive.json` e `testdata/himalayas.json` já commitados** (`d380b68`). Matéria
  -prima da Fase 1 pronta; as fontes ainda não existem no código.
- **Mapa de onde o Remote OK está assado** (base da Tarefa 1.1): seed de `sources` só tem
  `remoteok`; `fetchData` decodifica array cru no topo; `JobID.ID`/`Job` em `models.go` usam
  `id` (string), `position`, `company`. O Remotive responde **objeto** no topo, `id`
  **numérico** e outros nomes de campo. Quatro paredes → justificam a interface `Source`.
- Roadmap da Fase 1 escrito: 1.1 (segunda fonte + interface) em detalhe; 1.2–1.7 em resumo.

### 2026-10-03 — Tarefa 1.1 (Remotive + interface `Source`)

- ✅ **1.1 fechada.** `internal/sources/` com `Source` (3 métodos: `Fetch`, `ExternalID`, `ToJob`),
  `RemoteOK`, `Remotive` e factory `For(name)`. `models.go` apagado; `Config` agora carrega
  `[]sources.SourceConfig`. Migration `004_add_remotive.sql`. `TestFetch` cobre as duas fontes:
  100 (Remote OK, pós `items[1:]`) + 17 (Remotive) = 117; `runs` = 4 (2 fontes × 2 `run()`).
- **A API mente sobre si mesma.** `remotive.json` declara `"job-count": 18`, mas o array `jobs`
  tem 17. O `got 117` do teste estava **certo** — não havia vaga sumindo. Lição: campo
  auto-reportado de fonte externa se confere contando, nunca se confia.
- **`enable` vs `enabled`, de novo** (era 2026-09-09). Chave errada no `config.yaml` → campo no
  valor-zero (`false`) → Remotive pulado em silêncio, `exit=0`. `Load()` ainda usa
  `yaml.Unmarshal` cru, sem `KnownFields` — a guarda que transformaria o silêncio em erro
  continua **não** instalada. Ver D31.
- **WARN do `godotenv` rebaixado para `Debug`.** O ruído vinha do ramo de erro, não do `Load()`;
  no container o `.env` falta por design. Alarme que toca em toda subida saudável treina o olho
  a ignorar alarme.
- **Débitos de desenho abertos** (o aluno pulou o passo "na marra", então a conversa de desenho
  ficou para depois do verde): D28, D29, D30, D31. A pagar antes da 1.2.

### 2026-10-05 — diagnóstico (branch `feature/client`)

- 🔴 **B5 — o HEAD não compila.** `go build ./...` falha em `internal/sources/remoteok.go:17` e
  `himalayas.go:18` (chamam `HttpGetBody`; o client define `HTTPGetBody`). Contornando isso numa
  cópia descartável, aparece o terceiro erro: `main.go:239` usa `ctx` que não existe — o
  "contexto por fonte (60s)" descrito na retomada de 05/10 não está no código. Consequência: a
  suíte não roda e o `docker compose up --build` quebra. A retomada dizia "código pronto": **não está**.
  Nomes corrigidos pelo mentor a pedido (mecânico), junto com o typo `failed do build request`
  no client. O aluno trocou o `ctx` por `context.Background()` como andaime: compila, mas é
  justamente o "espera para sempre" do D7. Falta o contexto por fonte.
- 🟢 O commit `rascunho` mistura a 1.2 (fechada) com o WIP da 1.3.1. Mencionado uma vez.
- 🟡 **D35** registrado: `origin/main` tem um commit de docs (`d0296ea`) com marcador `=======`
  esquecido no `ROADMAP.md` e o `PROGRESS.md` de antes da 1.2.
- ✅ **1.3.1 fechada.** Dois prazos com papéis distintos: o client (30s, fixo) corta só o HTTP; o
  **orçamento da fonte** (parâmetro do `run`, 60s no `main`) corta todo o trabalho da fonte, banco
  incluso — vence quem estourar primeiro. `openRun`/`closeRun` ficam fora do orçamento: a run tem
  que abrir e fechar mesmo com ele vencido. `TestClientTimeout`: handler de 500ms que respeita
  `r.Context().Done()`, orçamento de 100ms, asserção sobre `runs` (1 linha, `fetched=0`, `error`
  com `context deadline exceeded`); 0.11s. Mutação (contexto sem prazo) → falha em 0.52s com
  `unexpected end of JSON input`. `COUNT(*) OVER()` traz contagem e linha numa query só.
  🔴 **B5 resolvido**, 🟡 **D7 pago**. (Pendências 2 e 3 do `/check` — variável da mensagem de
  falha e comentários em PT — feitas pelo mentor a pedido.)
- 🧠 **Asserção sobre o retorno do `run` testa o D21, não o timeout.** O `run` engole o erro da fonte
  (`Warn` + `continue` + `return nil`); o único rastro do timeout é `runs`. Mesma lição de 14/09.
- 🟢 Mencionados uma vez: `CollectionTimeOut` (Timeout é uma palavra; não precisa ser exportada),
  nome `TestClientTimeout` (testa o orçamento, não o client), `runErr` como `string` quebra no
  `NULL`, mensagem "jobs rows" numa query de `runs`, `err = run(...)` sobrescrito sem uso.

## Decisões tomadas

| Data       | Decisão                                                                            | Por quê                                                                                                                                                                   |
| ---------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-08-25 | Postgres como ponto de encontro Go ↔ Python                                        | Zero infra de mensageria; estado inspecionável; fila sai de graça de um índice parcial. Custo aceito: acoplamento por schema.                                             |
| 2026-08-25 | `jobs` = a vaga do mundo real, atualizada no lugar                                 | Mantém `digest`/`label`/`api` simples. Exige identidade estável própria em `jobs`.                                                                                        |
| 2026-08-25 | Mudança de conteúdo devolve `scored_at` para `NULL`                                | Nota calculada sobre texto velho não vale para texto novo.                                                                                                                |
| 2026-08-25 | Currículo como `cv.md` em volume montado — sem PDF, sem GUI                        | Container não tem display; parsing de PDF é custo sem aprendizado; markdown deixa currículo e vaga no mesmo formato, permitindo **um** extrator de skills em vez de dois. |
| 2026-08-25 | `analyzer` como container próprio no compose, com laço próprio                     | Coerente com o encontro via Postgres. Consequência a enfrentar na Etapa 3: duas instâncias pegam as mesmas linhas.                                                        |
| 2026-08-25 | Segredos (`DATABASE_URL`, `DISCORD_WEBHOOK_URL`) só em env; resto em `config.yaml` | Nada de segredo em arquivo versionado, desde o dia 1.                                                                                                                     |
| 2026-08-25 | Prometheus adiado da Etapa 1 para a 6                                              | Métrica sem scraper é log com nome bonito. `runs` cobre até lá.                                                                                                           |
| 2026-08-25 | Etapa 0 passa a atravessar Python também                                           | Integração adiada é integração que explode tarde. Custo: ~20 linhas de Python.                                                                                            |
| 2026-08-25 | Rotulagem contínua a partir do fim da Etapa 1                                      | 200 rótulos de uma vez é parede; e a Etapa 4 vai querer separar treino de teste.                                                                                          |
| 2026-09-03 | `runs.fetched` = itens que a fonte devolveu; `runs.failed` = itens que não viraram linha em `raw_jobs` por erro | Duplicata (`ON CONFLICT DO NOTHING`) é **sucesso**, não falha — senão `failed` dispararia todo dia e viraria ruído. Idem para o elemento 0 da resposta do Remote OK, que é o aviso legal de ToS e nunca tem `id`. `jobs` fica fora da conta porque `normalizeRawJobs` varre a tabela inteira, não só a run. |
| 2026-09-03 | Falha de `INSERT` em `raw_jobs` rebaixada de fatal para erro por item | Um item com dado ruim não deve derrubar a coleta inteira. **Custo aceito:** se o banco cair no meio, todos os itens falham e o coletor ainda termina com `exit=0` e `error=NULL` — verificado: `fetched=101, failed=100, exit=0`. Falta um limiar de `failed/fetched`. Dono: Etapa 1. |
| 2026-09-03 | Abrir a linha de `runs` **antes** do `getSourceID` | Fonte ausente de `sources` é erro de configuração que acontece de madrugada. Abrindo antes, ela aparece em `runs` com `source_id` NULL em vez de sumir sem rastro. |
| 2026-09-09 | Um container Postgres por **pacote** de teste (`TestMain`), não por teste | Subir Postgres custa ~5s; um por teste inviabilizaria a Tarefa 0.3. **Custo aceito:** os testes passam a compartilhar estado, e a partir da 0.3.2 (que escreve no banco) vai ser preciso isolar — limpeza entre testes, e nada de `t.Parallel()` sem isso. |
| 2026-09-09 | Transação cobre **só** `normalizeRawJobs` + o laço de `insertJob`; `insertRawJobs` fica fora | `raw_jobs` é append-only, idempotente por `ON CONFLICT DO NOTHING` e cada `INSERT` já é atômico — não há estado intermediário para alguém enxergar, e incluí-la só destruía trabalho bom (verificado: uma falha em `jobs` desfazia os 101 payloads). `jobs` entra porque é uma leitura seguida de N escritas derivadas dela, e é a tabela que o analyzer vai ler enquanto o coletor escreve — a transação existe para **consistência de leitura**, não para recuperação de falha (o sistema já se cura sozinho: `normalizeRawJobs` varre a tabela inteira). |
| 2026-09-09 | Dentro da transação, tudo-ou-nada: `insertJob` retorna no primeiro erro, sem `continue` | Postgres aborta a transação inteira no primeiro statement com erro (`25P02`) — o `continue` virava 245 erros falsos e o `Commit` falhava do mesmo jeito. É aceitável porque `jobs` é derivação **pura** de `raw_jobs`: nada se perde, a run seguinte reconstrói. **Custo aceito:** atraso até a próxima coleta, e o risco de pílula envenenada (D19). || 2026-09-15 | Identidade mora no `.env`; topologia mora no arquivo que descreve o ambiente | `user`/`senha` são invariantes (iguais no laptop e no container); `host:porta` varia com o observador. Grudadas numa string, a parte invariante é arrastada pela variante — foi o que quebrou o `go run .`. O `.env` guarda a URL do ponto de vista do terminal (`localhost:5433`); o `compose.yaml` monta a sua própria com `postgres:5432`, e `environment:` vence o `.env`. **Custo aceito:** o *molde* da URL fica escrito duas vezes — duplica-se o público, nunca o segredo. É a mesma linha que o k8s da Etapa 6 usa para separar `Secret` de manifesto. |
| 2026-09-15 | `image:` mantido junto de `build:` no serviço `app` | Com as duas chaves, o compose constrói **e** aplica a tag dada, em vez de derivar `job-searcher-app` do nome do diretório. A Etapa 6 vai referenciar essa imagem num manifest que não sabe em que pasta o build ocorreu. |
| 2026-09-15 | `config.yaml` assado na imagem, não montado como volume | Mantém a imagem autocontida: `docker run` funciona sem `-v`. **Custo aceito e não discutido pelo aluno:** mudar uma linha do `config.yaml` exige rebuild da imagem, o que atrita com a promessa do §6 do SPEC ("ajusta o `config.yaml` e roda `docker compose up`"). Revisitar na 0.4.3 ou na Etapa 1, quando a lista de fontes começar a mudar de verdade. |

---

## Diagnóstico do código existente (2026-08-25)

### 🔴 Blockers — corrigir antes de avançar

| #   | Onde                                         | Sintoma                                                                                                                                                                                   |
| --- | -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ~~B1~~ | `main.go` | ✅ **resolvido em 2026-08-26** (0.2.1). |
| ~~B2~~ | `db/migrations/002_add_sources.sql` | ✅ **resolvido em 2026-08-26** (0.1.1). |
| ~~B3~~ | `db/migrations/001_init.sql` (tabela `jobs`) | ✅ **resolvido em 2026-08-26** (0.1.2, migration `003`). |

### 🟡 Débitos — registrados, a pagar até o fim da etapa

| #   | Onde           | O quê                                                                                                                           | Status |
| --- | -------------- | ------------------------------------------------------------------------------------------------------------------------------- | ------ |
| ~~D1~~ | `go.mod`     | `pgx` e `godotenv` como `// indirect`.                                                                                          | ✅ pago 2026-08-26 (0.2.1) |
| ~~D2~~ | `main.go`    | `os.Exit(1)` espalhado pelo fluxo.                                                                                              | ✅ pago 2026-08-26 (0.2.1) — `run() error`, `main()` é o único ponto de morte |
| D3  | `main.go`      | Struct `Job` declarada dentro de `main()`. Aceitável hoje; vira problema quando a segunda fonte chegar (Etapa 1).               | ✅ pago 2026-09-03 — `Job`, `JobID` e `JobExternals` movidas para `models.go` |
| ~~D4~~ | `compose.yaml` | Credenciais do Postgres hardcoded no arquivo versionado.                                                                   | ✅ pago 2026-09-15 (0.4.1) — `${}` nos 4 pontos, incluindo o `-U`/`-d` do healthcheck; senha existe 1× no repositório |
| ~~D5~~ | —          | `testdata/*.json` existe mas nenhum teste usa.                                                                                  | ✅ pago 2026-09-14 (0.3.2) — `TestFetch` serve o fixture por `httptest.Server` |
| ~~D6~~ | `.env`         | Sem `.env.example` para quem clona. **Piorou em 2026-09-15:** eram 1 variável, agora são 4 — um clone recebe quatro interpolações vazias e um erro confuso. | ✅ pago 2026-09-30 (0.4.3) — `.env.example` com `DATABASE_URL` montada por `${}` |
| ~~D7~~  | `main.go:56`   | `http.Get` sem timeout — o padrão espera para sempre. No cron, um servidor que aceita a conexão e não responde pendura o coletor e o ciclo seguinte sobe outro em cima. | ✅ pago 2026-10-05 (1.3.1) — client com 30s + orçamento por fonte via contexto; provado por `TestClientTimeout` |
| D8  | `main.go:99`   | `content_hash` calculado sobre JSON compactado, mas `json.Compact` não canoniza ordem de chaves. Se a fonte reordenar chaves, todos os hashes mudam de uma vez e nascem "novas versões" falsas. | aberto — sem dono; sintoma seria inexplicável se ocorrer |
| D9  | `main.go:86`   | Falha de `Unmarshal` é logada sem identificar o item — e não há como identificar, já que o parse é justamente o que falhou. Alarme sem endereço. | aberto — reavaliar em 0.2.3 junto com `runs.failed` |
| D10 | dados          | Payloads do Remote OK chegam com entidades HTML cruas (`&amp;`) e mojibake (`Donâ\u0080\u0099t`, `å\u008C\u0096`). Um `slug` vazio. `salary_min: 0` significa "não informado", não zero. | aberto — dono: Etapa 2, na normalização de texto |
| D11 | `main.go:54`   | `defer` roda durante panic e grava `error=NULL`: a run mente dizendo sucesso enquanto o processo morreu de SIGSEGV. Verificado em 2026-09-03. | aberto — dono: Etapa 1 |
| D12 | `db/migrations/001_init.sql` | `runs` não tem `finished_at` — dá para saber quando começou e o que houve, não quanto durou. E não há limiar de `failed/fetched` que marque a run como fracasso. | aberto — dono: Etapa 1, migration `004` |
| D13 | `main_test.go:143` | `err := db.Close()` é sobrescrito pela linha seguinte: se o `Close` falhar e o `Terminate` funcionar, `teardown()` devolve `nil`. O `return err` final é sempre `nil`. | ✅ pago 2026-09-09 — `errors.Join` acumulando os dois |
| D14 | `main_test.go:90` | Loop `for rows.Next()` sem checar `rows.Err()` depois. | ✅ pago 2026-09-09 |
| D15 | `main.go:230` | `defer rows.Close()` vinha antes do `if err != nil`. Não quebra em pgx (`Query` sempre devolve `rows` não-nil), mas quebraria em `database/sql`. | ✅ pago 2026-09-09 |
| D16 | `main.go` | Nenhuma transação no projeto, apesar de "transação" estar nos conceitos da Tarefa 0.2. | ✅ pago 2026-09-09 — ver decisão da fronteira |
| D17 | `Makefile:2,4,6` | Caminho absoluto `/mnt/ssd_m2/...` nos três alvos do goose: quebra em qualquer outra máquina, inclusive no CI da Etapa 7. Senha repetida três vezes (D4 com outro rosto). | 🔶 parcial — caminho relativo desde 2026-09-09; a DSN literal repetida 3× continua aberta (ver D4) |
| ~~D18~~ | `main.go:246` | `logger.Info` por item: ~100 linhas INFO por execução escondendo as poucas que importam. `runs` já guarda a contagem. | ✅ pago 2026-09-30 (0.5.1) — `Debug` por item + `source collected` por fonte |
| D19 | `main.go` | Pílula envenenada: com tudo-ou-nada no laço de `insertJob`, um item permanentemente inválido faz **toda** run futura falhar, já que ele reaparece a cada coleta. Saída conhecida: savepoint por item (`tx.Begin()` sobre uma `Tx`). | aberto — dono: Etapa 1, quando houver 3 fontes e frequência medida |
| ~~D20~~ | `main.go:102` | `processSource` sem resultado nomeado: o `defer` lê o `err` **local**, então `runs.error` grava o erro cru sem o `fmt.Errorf` que envolve o retorno (verificado: `WARN` diz `failed to get source id: ...`, `runs.error` diz só `failed to query db: ...`). No dia que houver um `return fmt.Errorf("algo")` sem `err`, o `defer` grava `error=NULL` — D11 por outro caminho. | ✅ pago 2026-09-30 (0.5.1) — resultado nomeado; verificado com fonte `fake` (run 45) |
| D21 | `main.go:99` | Se **todas** as fontes falharem, `run()` devolve `nil` e o processo sai com 0. Segundo caminho de falha silenciosa, irmão do D12 (limiar de `failed/fetched`). **Visto em 2026-09-30:** clone sem schema → `app` sai 0 → `analyzer` sobe e quebra. | aberto — dono: Etapa 1 |
| D23 | `compose.yaml:15` | `container_name: postgres_db` fixa o nome fora do namespace do projeto: impede subir uma segunda instância (`docker compose -p outro up`) e, portanto, impede testar a corrida do healthcheck sem destruir o volume de dados real. | aberto — dono: Etapa 6, quando houver mais de um ambiente |
| D24 | `compose.yaml:26` | Healthcheck usa `$POSTGRES_USER` (expandido pelo **Compose**, em tempo de parse) em vez de `$$POSTGRES_USER` (expandido pelo shell do container, em tempo de execução). Funciona, mas o valor vaza literal no `docker inspect` e o padrão não sobrevive ao k8s, onde não há Compose para interpolar. | aberto — 🟢, dono: Etapa 6 |
| ~~D27~~ | `analyzer/main.py:1` | Shebang num arquivo sem bit de execução no git (`100644`): `ruff check` falhava com `EXE001` em clone. | ✅ pago — shebang removido; `ruff` limpo em ambiente novo. Confirmado no diagnóstico de 2026-10-02 |
| D25 | `compose.yaml` (`app`, `analyzer`) | `build:` + `image:` com tag fixa: o `up` só constrói se a tag **não existir**, e tag é global no Docker, não por projeto. Verificado em 2026-09-30: o clone limpo da 0.4.3 rodou a imagem de 29/09, sem o `migrate`. Mitigação hoje: `--build` após mudar código. | aberto — reavaliar na Fase 6 (tags versionadas) |
| D26 | `analyzer/main.py:22` | Erro de query (ex.: `UndefinedTable`) sai como traceback cru; só a falha de conexão tem mensagem de uma linha. | aberto — 🟢, pagar quando o analyzer ganhar lógica real |
| ~~D22~~ | `main.go:21` | `var source = "https://remoteok.com/api"` morta desde a chegada do `config.yaml`, e sombreada pela variável do laço. Junto: `Url` → `URL`, `runId` → `runID`. | ✅ pago 2026-09-30 (0.5.1, feito pelo mentor a pedido — mecânico) |
| ~~D28~~ | `internal/sources/sources.go:24` | A interface `Source` mora no pacote que a **implementa**, não no que a **consome** (`main`). Viola o §8 ("interfaces pequenas, definidas no consumidor"). | ✅ pago 2026-10-03 — interface `Source` + `For` movidos para o `main`; `sources` exporta só tipos concretos + `Job`; "accept interfaces, return concrete types". O aluno tentou subir `Job` junto para o `main` (o anti-move que o mentor sinalizou) e o import cycle provou a regra: interface sobe, dado desce. |
| ~~D29~~ | `main.go` (`collectAndStoreData`) | Transação por item decorativa (envolvia um único `Exec`) + `defer tx.Rollback` dentro do `for`. | ✅ pago 2026-10-03 — decisão **(a)**: transação removida, `insertJob` roda solto, `defer` fora do laço. Erro **de item** (`compactJSON`, `ToJob`, `insertJob`, `insertRawJobs`) agora é `Warn`+`failed++`+`continue`, não `return` — isso realiza a isolação de falha que (a) prometia e evita a pílula envenenada que o `return` reintroduzia. Falha sistêmica aparece como `failed==fetched` (dono: D21/D12). |
| ~~D30~~ | `main.go` (`collectAndStoreData`) | `err` do `compactJson` sobrescrito antes de ser checado. | ✅ pago 2026-10-03 — erro checado antes de usar o `hash`; `compactJson` → `compactJSON` (inicialismo). |
| ~~D31~~ | `main.go` (`Load`) | `yaml.Unmarshal` cru, sem `KnownFields`: chave desconhecida ignorada em silêncio (o `enable`×`enabled`). | ✅ pago 2026-10-03 — `yaml.NewDecoder` + `KnownFields(true)`: chave desconhecida no `config.yaml` agora é erro de parse, não comportamento faltando. Fecha a **classe** do bug. (D29-final e D31 digitados pelo mentor a pedido do aluno — ele descreveu a solução, sem tempo de digitar.) |
| D32 | `main.go` (`collectAndStoreData`) | O refactor da 1.1 trocou o `normalizeRawJobs` (varredura idempotente de `raw_jobs` → `jobs`) por "normalizar só o item recém-inserido + `if existed continue`". Dois efeitos: (1) `jobs` deixou de se auto-curar — `raw_job` que grava mas cujo `insertJob` falha vira **órfão** permanente (na run seguinte cai no `existed`); (2) matou o reprocessamento prometido no SPEC §4.1 ("rodar o normalizador de novo sobre o histórico"). | aberto — dono: Fase 2 (quando o normalizador melhorar); conserto provável = normalização como passo idempotente separado sobre `raw_jobs` |
| D33 | `main.go` (`insertJob`) | O modelo `sources.Job` carrega `Date`, `Location` e `Tags`, mapeados com cuidado nas três fontes (incl. 3 formatos de data), mas `insertJob` grava só `title/company/url/description`. `Date` é calculado e descartado **mesmo havendo coluna `posted_at`**; `Location`/`Tags` não têm coluna ainda. Owners futuros concretos: Fase 2 (filtro de localização lê `Location`), Fase 3a ("idade da vaga" lê `posted_at`). | aberto — 🟡; migration + colunas quando os donos chegarem |
| ~~D34~~ | `internal/sources/himalayas.go:51` | Rótulo de erro copiado do Remotive + typo `localtion`. | ✅ pago 2026-10-03 (mentor, mecânico) — `remotive:` → `himalayas:`, `localtion` → `location`. |
| D35 | `origin/main` (`d0296ea`) | Commit `docs: change docs` com marcador de conflito `=======` esquecido no `ROADMAP.md` (linha 232, com o roadmap antigo da Fase 1 colado embaixo) e `PROGRESS.md` de volta ao estado pré-1.2. O merge de `feature/client` vai conflitar nos dois arquivos. | aberto — resolver no merge: a versão certa dos docs é a do `feature/client` |

### 🟢 Nits — mencionados uma vez

- ~~`main.go:39` usa `slog.Info` direto enquanto o resto usa `logger`~~ → resolvido em 0.2.1.
- ~~`godotenv.Load()` falhando só loga e segue, implicitamente~~ → resolvido em 0.2.1: agora é `logger.Warn` explícito.

---

## O que aprendi

- **2026-08-26** — `ON CONFLICT` usa a restrição `unique` como mecanismo, não como
  validação: elimina a janela de corrida que um `SELECT` antes do `INSERT` deixaria aberta.
- **2026-08-26** — `CASCADE` é comportamento declarado na definição da FK, não parâmetro
  do `DELETE`. E aqui seria destrutivo: apagaria o histórico de `raw_jobs` em silêncio.
  Uma FK que barra o `DELETE` está trabalhando a favor.
- **2026-08-26** — sequência de `serial` não recua quando a linha é deletada. Não importa
  aqui porque o código busca `sources` pelo nome (chave natural), não pelo id.
- **2026-08-26** — chave natural × chave substituta coexistem de propósito: a substituta
  (`jobs.id`) é o que o resto do schema referencia; a natural (`source_id, external_id`) é
  como se **reencontra** a linha quando o dado chega de novo da fonte.
- **2026-08-26** — FK é para coluna que aponta para uma **linha** de tabela que o sistema
  possui. `external_id` é um **valor** vindo de fora — nenhuma tabela é dona da lista de ids
  do Remote OK, então não leva FK.
- **2026-08-26** — `UNIQUE` sem `NOT NULL` é decorativo: `NULL` não é igual a `NULL`, então a
  constraint não afirma nada sobre duas linhas com identidade ausente e deixa passar.
- **2026-08-26** — `down` é o inverso do **schema**, nunca do **dado**. `DROP COLUMN` destrói
  informação que o `up` seguinte não consegue reconstruir; em banco vazio a reversibilidade
  é ilusão de cronograma.
- **2026-09-09** — `TestMain` e `*testing.T` são mundos separados: o runtime do `testing`
  chama os testes com assinatura fixa, então o único canal entre eles é **escopo de pacote**.
  Estado global é aceitável num `_test.go` porque o pacote de teste tem um dono só e um
  ciclo de vida só — em produção, não.
- **2026-09-09** — `defer` amarra a limpeza à função que o **declarou**, não ao recurso.
  Quando quem cria e quem usa são funções diferentes, o `defer` mente; devolver um
  `teardown` (junto do erro, para cobrir a falha no meio do setup) põe a decisão em quem
  conhece o ciclo de vida.
- **2026-09-09** — um teste que passa não prova nada até você fazê-lo falhar de propósito.
  Mutação é o `/check` do próprio teste.
- **2026-09-09** — o rollback dispara por causa do `return`, não do erro. Erro engolido
  com `continue` não aciona o `defer` — mas já envenenou a transação por dentro: em Go
  parece que deu certo, no Postgres nada mais funciona (`25P02`).
- **2026-09-09** — transação para **consistência de leitura** ≠ transação para
  **recuperação de falha**. Se o dado de origem está fora do escopo desfeito, tudo-ou-nada
  não custa dado, custa atraso.
- **2026-09-09** — `defer` se liga à função, não ao bloco. Um `for` não tem ponto de
  retorno, então `defer` dentro dele só enfileira. Para limpeza por iteração, o corpo do
  laço precisa virar função.
- **2026-09-09** — YAML ignora campo desconhecido em silêncio: config escrito errado não
  vira erro, vira comportamento faltando. `KnownFields` no decoder do `yaml.v3` transforma
  isso em erro.
- **2026-09-14** — teste que só compara "antes vs. depois" aprova um coletor que não faz
  nada: zero concorda com zero. Valor absoluto e comparação relativa medem coisas
  diferentes; nenhum dos dois substitui o outro.
- **2026-09-14** — quando uma transação desfaz, o banco fica idêntico ao estado anterior.
  Nenhuma contagem distingue "não precisou mudar" de "tentou e falhou" — só o registro de
  execução distingue. É para isso que a tabela `runs` existe.- **2026-09-15** — `FROM` num Dockerfile não continua o estágio anterior, **corta** com ele. O
  estágio final nasce de uma base limpa e só recebe o que um `COPY --from` carrega. Duas
  cozinhas: o que você não levar na mão fica para trás, inclusive o sal.
- **2026-09-15** — `ports: "5433:5432"` é porta de rua, não número de sala. Quem já está
  dentro da rede do compose fala direto com a porta interna; o mapeamento existe só para
  quem vem de fora. O `5433` é arbitrário e resolve um problema exclusivo do laptop (a 5432
  já está ocupada lá).
- **2026-09-15** — critério de aceite não é "parou de dar erro", é **qual** erro. O erro de
  conexão foi a *prova* de que o `config.yaml` tinha sido lido: só chega ao `openConn` quem
  passou pelo `Load`. Ler a mensagem, não o status de saída.
- **2026-09-15** — ferramenta que **resolve** nome (`docker run`) protege você; ferramenta
  que **filtra** nome (`.dockerignore`, `.gitignore`, tag de YAML, variável de ambiente) não
  tem como distinguir typo de ausência legítima. Em filtro, verifica-se o resultado.
- **2026-09-15** — identidade é invariante, topologia é variante. Uma string que gruda as
  duas só consegue atender um observador por vez.

---

## Retomada rápida

**1.3.1 fechada em 2026-10-05.** Próximo: **1.3.2 — D21** (todas as fontes falhando → `exit≠0`).
Depois, na 1.3: D12 (`finished_at` + limiar `failed/fetched`), D11 (`defer`/panic grava
`error=NULL`) e o `if err != nil` morto no `processSource` (resolve junto do D21). Quando o D21
for pago, o `TestClientTimeout` pode voltar a afirmar sobre o retorno do `run`. Fora da 1.3: D32
(auto-cura/reprocessamento), D33 (`Date`/`Location`/`Tags` não persistidos), D35 (docs do `main`).

Estado verificado em 2026-10-05: `go build`/`go vet`/`gofmt` limpos, suíte verde
(`TestFetch` 137/137/6, `TestClientTimeout` 0.11s). Depois de mudar código, usar `docker compose up --build` (D25). A DSN do terminal
usa a porta `5433` (ver `.env.example`).
