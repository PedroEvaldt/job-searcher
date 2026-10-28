# PROGRESS — job-radar

> Estado entre sessões. Atualizado a cada `/check` aprovado e a cada `/pausa`.

---

## Onde estou

**Tarefa 1.6 fechada em 2026-10-16** — o critério da Fase 1 ("roda 7 dias sozinho") foi atendido no
Pi: 2026-10-08 17:56 → 2026-10-16, as 3 fontes todos os dias, maior intervalo 6h03, três quedas de
energia atravessadas sem intervenção. Ordem da Fase 1: **1.6 → 1.5 → 1.7**. Dados do Pi copiados
para o `jobs` local em 2026-10-16 (105 runs, 12.584 vagas; dump em `db_dump/`, ignorado pelo git).
**Tarefa 1.5 fechada em 2026-10-18** — dedupe no ar no Pi (`26cccd7`): marcação na gravação +
histórico automático; consulta de aceite `0 | 0` entre as vagas vivas. Tarefa 1.7 (fechamento da
Fase 1): **1.7.1 (D37)**, **1.7.2 (D39)** e **1.7.3 (README + D17)** fechadas em 2026-10-18. **Fase 1
fechada em 2026-10-18.** Auditoria de fim de fase feita (sem 🔴).

**Fase 2 aberta em 2026-10-18** (EDA em Python). **Tarefa 2.1 (bancada de EDA) fechada em
2026-10-19.** **Tarefa 2.2 (defeitos do dado) fechada em 2026-10-20**: `docs/eda.md` com 7 defeitos.
**Tarefa 2.3 (boilerplate) fechada em 2026-10-21**: `analyzer/text.py` (HTML → parágrafos) e
`analyzer/boilerplate.py` (contagem + corte, k = 10 descrições distintas), 20 testes.
**Tarefa 2.4 fechada em 2026-10-22** — 2.4.1 (localização, `eda/05_location.py`), 2.4.2 (idioma,
`eda/06_language.py`) e 2.4.3 (seção `# Policies` no `docs/eda.md`), as três escritas pelo mentor a
pedido (insistência, §0). Dado do Pi re-copiado para o banco local **na 5433** (`.env` mudado de
5432 → 5433 a pedido): **20.817 vagas, 20.029 vivas** (antes 15.279/14.714). Achado-chave: PT é
2,8% no total mas **18% dentro do `accept`** → lote não-inglês se mantém; limitações viraram
**D45**. **Tarefa 2.5 e Fase 2 fechadas em 2026-10-22**: decisão em conjunto — **todos os débitos
reagendados**, consumer-driven (D10/D32/D33 → 3a, D42 → backlog, D41 → 3a/5, D44 → 5, D45 → 4b;
donos atualizados no catálogo); README da parte Python escrito (`analyzer/README.md`). Pi
com a imagem de `8498ca2` desde 2026-10-18 20:00 UTC (D37 e D39 em produção).

**Fase 3a aberta em 2026-10-22** (extração no código; roadmap detalhado no `ROADMAP.md`:
3a.1 extrator de skills → 3a.2 nas vagas → 3a.3 normalização Go D32/D33/D10 → 3a.4 salário/anos/
idade → 3a.5 TF-IDF → 3a.6 fechamento). **Tarefa 3a.1 fechada em 2026-10-23.** `cv.md` na raiz
(dado do aluno, escrito pelo mentor a pedido; ainda **não gitignorado** — decisão do aluno de
versionar por ora) e `analyzer/skills.py`: `ALIASES` (forma escrita → canônico; universo =
`set(ALIASES.values())`) + `skills_extractor(text, skills) -> dict[str, float]` (dicionário/regex
com `(?<!\w)…(?!\w)`, soma contagem de menções no canônico). Testes em
`tests/test_skills_extractor.py` (7 casos). A lógica central foi do aluno; `cv.md`, `ALIASES` e a
migração dos testes a pedido. **Pendências levadas à 3a.2:** confiança é contagem crua (não
normalizada, infla com texto longo); fechar um comentário documentando o esquema de confiança.
**Subtarefa 3a.2.1 fechada em 2026-10-23:** `eda/07_skills.py` roda o pipeline
(`text.py`→`boilerplate.py`→`skills.py`) sobre as 20.029 vivas e imprime presença por skill (Go
2665, Python 2272, …, C++ 282). Achado que reprioriza a 3a.3: **D10 não afeta a extração de
skills** (Remote OK é 0,7% das vagas; mojibake/entidades estragam não-ASCII, skills são ASCII) →
o motor da 3a.3 vira o **D33** (salário/`posted_at` sem coluna preenchida), não a limpeza de
texto. **Subtarefa 3a.2.2 fechada em 2026-10-23:** `main.py` extrai e grava `job_skills` (cursor, `%s`,
`executemany`, `ON CONFLICT (job_id, skill) DO UPDATE`), chamado pelo `main` num `with conn:`;
erro de banco → exit 1. 13.296 pares / 6.434 vagas; idempotente no caminho feliz (rodar 2× mantém
a contagem). `skills_per_job` levantado pro `main.py` (sem pandas em produção; segue duplicado no
`eda/07_skills.py` — pressão pra extrair pra módulo próprio). `ALIASES` expandido a pedido para
103 formas (~70 skills canônicas). **Dívidas conhecidas da 3a.2.x:** (1) `DO UPDATE` não remove
skill que sumiu quando a descrição muda; (2) confiança = contagem crua (→ pesar na 3a.5);
(3) matcher sem longest-match (C/R fora; `Shell` casa em "Bash shell"). **Subtarefa 3a.2.3 e
Tarefa 3a.2 fechadas em 2026-10-23:** migração `008` (`skills_extracted_at` como marcador de fila,
separado do `scored_at`); laço claim→processa→marca→commit por lote com `with conn.transaction()`;
drenou as 20.029 vivas (4m46), re-run no-op (16s). **Gotcha do psycopg (vale pra Fase 5):** leitura
antes do laço com `autocommit=False` abre transação implícita → `with conn.transaction()` vira
savepoint, travas `FOR UPDATE` presas até o fim; `conn.autocommit = True` resolve (commits por lote
reais). A extração de skills da Fase 3 está, portanto, completa e idempotente, com concorrência.
**Fase 3a.3 aberta 2026-10-23. Subtarefa 3a.3.1 fechada:** `insertJob` grava `posted_at` a partir
do `Date` (ponteiro-na-fronteira, zero-time → `NULL`); `TestNilPostedAt` (código do aluno, teste
reescrito pelo mentor). Forward-only — as 20k existentes seguem com `posted_at` NULL até a
varredura. **Subtarefa 3a.3.2 fechada 2026-10-23:** `normalizeRawJobs` — varredura idempotente,
última versão por identidade (`ROW_NUMBER`) com cursor no `id`, **lê lote → fecha `rows` →
processa** (pgx dá `conn busy` se `insertJob` roda dentro do `rows.Next()`), re-parseia com a fonte
real via `newSource`. Flag `-renormalize` (one-shot). `TestNormalizeRawJobs` com Remote OK real —
lição: a `jobSource` grava formato de `sources.Job` que o reprocessamento via `newSource` não lê (o
Himalayas `time.Unix(0,0)=1970` apareceu). Varredura/flag do aluno; conn-busy + teste pelo mentor.
Suíte verde. **Subtarefa 3a.3.3 e Tarefa 3a.3 fechadas 2026-10-23:** salário extraído pra colunas
nuláveis — `SalaryMin`/`SalaryMax` → `*float64`, `Currency`/`SalaryPeriod` → `*string`, helpers
`nonZero`/`nonEmpty` (`0`/`""` → `nil` → `NULL`, defeito 5). Remote OK (min/max) e Himalayas
(min/max/currency/period) extraem; Remotive deixa salário `NULL` (texto livre → Python/3a.4,
defeito 6). Migração `009` (`salary_period` — nome do arquivo diz "duration", conteúdo é "period";
cosmético). `TestRemoteOKSalary` (sentinela + decimal `0.59`). Estrutura do aluno; tipos/teste pelo
mentor. **Normalização Go (D32/D33) da Fase 3a completa.**

**Tarefa 3a.4 fechada 2026-10-25** (extração estruturada em Python): 3a.4.1 idade (`eda/08_date.py`,
subtração vetorizada; 100% das vivas com `posted_at`, sem sentinela 1970 no dado); 3a.4.2 salário
(`salary.py`: `normalize_salary` → USD anual, `TO_ANNUAL`/`TO_USD` 26 moedas, sentinela → `None`, 10
testes); 3a.4.3 anos (`experience.py`: `extract_years` por regex, 6 testes). `ruff`/`mypy --strict`/
43 testes verdes. **Lição que pegou:** `ruff`+`mypy` acharam um bug de refactor no `main.py`
(`skills_per_job` com nomes indefinidos) invisível aos testes → estática antes do `/check`.
**Tarefa 3a.5 fechada 2026-10-26** (ranking TF-IDF — capstone da 3a): 3a.5.1 (`eda/11_similarity.py`,
cosseno `cv` × vagas, top-15 DevOps/Python, max sim ~0.2) e 3a.5.2 (`insert_scores`/`scorer` no
`main.py`: corpus inteiro vetorizado de uma vez, CV = doc 0, `executemany` + `ON CONFLICT DO UPDATE`,
`model_version='tfidf-v1'`, `scored_at=now()`; idempotente). `sklearn` virou dep de produção →
override de `mypy` no `pyproject.toml`. Furos do `/check` corrigidos pelo aluno (cursor fechado,
`enumerate` invertido, `float()` do `np.float32`, typos no SQL/`model_version`, `scored_at` ausente,
params do `executemany`). **Pendência de verificação:** runtime (idempotência + top-10) reportado
pelo aluno; o mentor não reproduziu — banco local desta máquina na migração **7** (sem `008`/`009`),
`insert_job_skills` quebra em `skills_extracted_at`.

**Tarefa 3a.6 e Fase 3a fechadas em 2026-10-26.** Auditoria de débitos: D32 pago, D33 parcial
(salário/`posted_at` pagos; `Location`/`Tags` → Fase 5), D10 → Fase 4b, **D46 pago** (skills
refeitas quando o texto muda: `insertJob` reseta `skills_extracted_at` como faz com o `scored_at` +
`insert_job_skills` apaga antes de reinserir; `TestNilSkillsExtractedAt`), D47 (confiança crua) e
D48 (longest-match) → Fase 4c. README atualizado (`analyzer/README.md` + raiz). Sem 🔴. Pendência de
verificação que atravessa a fase: o mentor não reproduz runtime nesta máquina (Docker sem socket;
banco da 5433 na migração 7 / intermitente) — testcontainers Go e observações de banco ficaram no
relato do aluno.

**Fase 3b aberta em 2026-10-26, fechada em 2026-10-28** (rotulagem e avaliação; roadmap detalhado no `ROADMAP.md`:
3b.1 binário `radar` → 3b.2 rotular + congelar split → 3b.3 TF-IDF no conjunto congelado → 3b.4
fechamento). **Subtarefa 3b.1.1 fechada em 2026-10-26:** `cmd/radar/main.go` (`package main`) — o
segundo binário Go nasce; `pgx.Connect`+`Ping`, contagem `LEFT JOIN labels … WHERE duplicate_of IS
NULL`, rodou contra o banco (`LABELED 0 / 16204`, confere com o `psql`). Conexão ainda duplicada de
propósito (SPEC §7: `internal/store`/`internal/config` só quando a duplicação doer). Lição que
pegou: o que faz um pacote virar executável é o `package main`, não a pasta `cmd/radar/`.
**Subtarefa 3b.1.2 fechada em 2026-10-27:** `radar label` (`cmd/radar/main.go`) mostra uma vaga por
vez e grava em `labels` (`would_apply` + opcionais), sem nunca exibir score (invariante §2.1). Uma
vaga por vez com `QueryRow` (evita o `conn busy` da 3a.3.2). Verificado contra o banco (2 rótulos de
teste, `seniority`≠`note`, vazios → `NULL`, `status` subiu, apagados depois). **Nota de processo:** o
aluno editou o §0 do `CLAUDE.md` pra permitir que o mentor escreva o código na insistência (hoje em
conflito com o §8, a alinhar); os três furos finais (aliasing `seniority`/`note` no mesmo `&v`, trim
no armazenamento, `ErrNoRows` como fim normal) foram escritos pelo mentor sob essa regra. Nit aberto:
Ctrl+D (`io.EOF`) loga como erro em vez de sair quieto. **Próximo: 3b.1.3** (robustez da sessão:
retomar, pular, sair limpo) ou direto pra **3b.2** (rotular ~100 + congelar o split).
Banco local na 5433; `scores` com `tfidf-v1`. **Pendência de ambiente:** `go test ./...` (collector)
precisa de Docker pro testcontainer; o mentor não roda aqui (socket negado).

**2026-10-27 — campos auxiliares de `labels` preenchidos pelo mentor (NÃO é gabarito humano).** A pedido
insistente do aluno (após ele editar o §0 do `CLAUDE.md`), o mentor leu título+descrição das 51 vagas
rotuladas e preencheu `hires_latam`/`seniority` **só quando o texto era explícito**: `seniority` por
palavra de nível no título (senior/sr/principal/vp/director/head/lead; junior/student/"no experience");
`hires_latam` por LATAM/global explícito (→true) ou exigência de país dura (→false). Resultado:
`hires_latam` 19/51, `seniority` 24/51; resto `NULL`. Também preencheu `note` (resumo de 1 frase por
vaga) nas 49 que estavam vazias; as 2 notas do próprio aluno (vagas 1 e 6) foram preservadas. **Importante p/ a avaliação:** só `would_apply` é
gabarito humano; `hires_latam`/`seniority` são **extração de modelo, não reproduzível, sem `model_version`**
— não devem virar alvo nem entrar como verdade no conjunto congelado (3b.2.2). Os 5 `latam` + 3 `sen` que o
aluno já tinha marcado à mão foram preservados (guarda `IS NULL`). Caminho certo p/ esses sinais segue sendo
extração por código (features) ou o Jev (4b), não esta passada manual.

**2026-10-27 (cont.) — mesma passada estendida às vagas 52–122.** O aluno rotulou mais vagas (`would_apply`,
total agora **122**) e pediu o mesmo preenchimento auxiliar nas 71 novas. Mesmas regras explícito-only, mesma
guarda `IS NULL` (preserva o que o aluno pôs à mão). A vaga **88** é só uma imagem base64 sem texto → ficou
sem `hires_latam`/`seniority`. Totais acumulados nas 122: `note` 122/122, `hires_latam` 61/122, `seniority`
57/122. Vale o mesmo aviso: só `would_apply` é gabarito; os auxiliares são extração de modelo e não entram no
conjunto congelado.

**2026-10-28 — mesma passada estendida às vagas 123–250 (antes de recongelar).** O aluno rotulou mais 128 vagas
(`would_apply`, total agora **250**) para engrossar o conjunto antes de começar a 4a, e pediu o preenchimento
auxiliar nas novas. Mesmas regras explícito-only, mesma guarda `IS NULL` (0 valores humanos sobrescritos). As
128 novas receberam: `note` 128/128, `hires_latam` 78/128 (11 `true` LatAm explícito — Brasil CLT/estágio/PT-BR,
território/residência LatAm, México, "work from anywhere" — e 67 `false` por país/região dura), `seniority`
51/128 (só por palavra de nível no título). Totais nas 250: `note` 250/250, `hires_latam` 139/250, `seniority`
108/250 (22 junior, 3 mid-level, 83 senior). Mesmo aviso de sempre: só `would_apply` é gabarito. **Próximo passo
do aluno:** subir `version` para `test-set-v2` e rodar `freeze_test_set.py` (régua nova, membros totalmente
diferentes), depois `evaluate_baseline.py` para o novo chão do `tfidf-v1` na v2.

**2026-10-27 — 3b.2.2 fechada: conjunto de teste congelado.** `analyzer/freeze_test_set.py` lê os 122
rótulos (`ORDER BY job_id`), faz `train_test_split` estratificado em `would_apply` com `random_state=7`,
`test_size=0.3`, e grava o conjunto de teste em `analyzer/frozen.json` (`test-set-v1`). Resultado: **37**
de 122 no teste (**5** positivos / 32 negativos; treino fica com 11 positivos / 79). `test_set` ordenado por
`job_id` → diff estável; rodar de novo gera arquivo idêntico. O arquivo commitado **é** o congelamento;
treino = tudo que não está nele. Pendências do `/check` corrigidas pelo mentor a pedido insistente do aluno
(materializar as listas uma vez + `stratify` sobre a mesma lista, ordenar o teste, propagar o código de
saída, `with` na conexão). Nota de ambiente: `ruff` acusa `EXE002` em todo `.py` por causa do mount FUSE
(mostra tudo `0755`); o git grava `100644`, então some no checkout limpo.

**2026-10-27 — 3b.3.1 fechada: baseline TF-IDF medida no conjunto congelado.**
`analyzer/evaluate_baseline.py` lê os ids do `frozen.json`, busca o `score` de `tfidf-v1` numa query só
(`job_id = ANY(%s)`), e reporta `roc_auc_score` com a versão e o tamanho: **`tfidf-v1 ROC AUC=0.5000
n=37 pos=5`**. Conferido na mão (positivos nos ranks 1/7/24/29/34 → 80/160 = 0.5 exato): não é bug nem
empate, é achado honesto — o cosseno com o currículo **não rankeia** as vagas que o aluno aplicaria acima
das que não, neste conjunto. É o número que justifica embeddings (4a) e o Jev (4b). Polimento feito pelo
mentor a pedido (constante `MODEL_VERSION`, query única, checagem explícita de score faltando no lugar de
`assert`, métrica arredondada, `__name__` guard). ruff/mypy limpos.

**2026-10-28 — 3b.1.3 e 3b.4 fechadas: Fase 3b encerrada.** A 3b.1.3 estava com três de quatro itens já
prontos pela estrutura do `label()`: a query `WHERE l.labeled_at IS NULL ORDER BY j.id` retoma nas
não-rotuladas e nunca re-mostra uma rotulada; faltava só sair limpo. Commitado o trabalho que estava solto
no `cmd/radar/main.go` (commit `3045c1e`): Ctrl+D (`io.EOF`) sai com `"shutting down"` em cada prompt em vez
de logar erro (fecha o nit da 3b.1.2), `cleanDescription` + `clearScreen` deixam uma vaga por tela (só
exibição; o texto no banco não muda). **"Pular vaga" cortado por YAGNI** — as 122 foram rotuladas sem
precisar; se fizer falta, vira subtarefa de ~20 min. A fase não abriu nenhum 🔴/🟡; o único nit morreu com
esse commit. `.gitignore` passou a ignorar o binário `cmd/radar/radar` e os `.fuse_hidden*` do mount FUSE
(commit `125fa2d`). Docs: README raiz ganhou "Labelling jobs (`radar`)" + Phase 3b no status;
`analyzer/README.md` ganhou "Evaluation: the frozen test set" (freeze + baseline). Build e `go vet` limpos.
**Próxima fase: 4a — embeddings** (sentence-transformers + pgvector contra a baseline `ROC AUC=0.50` no
conjunto congelado).

Última sessão: **2026-10-28**.

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
- ✅ **1.3.2 fechada (D21).** Regra: o `run` devolve erro quando **nenhuma** fonte ligada termina com
  sucesso (`For` ok + `processSource` nil). Erros juntos com `errors.Join`, cada um envolvido com o
  nome da fonte, e o conjunto com `no source succeeded: %w`. Zero ligadas → erro próprio (o `Join`
  de lista vazia é `nil`; e `fmt.Errorf("%w", nil)` não é `nil`, imprime `%!w(<nil>)` — o exit 1
  saiu certo por acaso na primeira versão). `TestClientTimeout` voltou a afirmar sobre o retorno,
  com `errors.Is(err, context.DeadlineExceeded)` atravessando `%w` + `Join`; `TestPartialFailure`
  (remotive bom + remoteok com 500) → `nil`, 2 runs, 1 com erro. Mutações: "erro em qualquer falha"
  → `TestPartialFailure` falha; "sem prazo" → `TestClientTimeout` falha **mesmo com o `run`
  devolvendo erro** — só o `errors.Is` distingue timeout de outra falha. `go run .` com zero
  ligadas → `exit 1`, `no source succeded: no source was activated`.
- 🟢 Mantidos por decisão do aluno / mencionados uma vez: bloco `failed == fetched` comentado no
  `processSource` (volta no D12), typos `succeded`/`activated`, resto de roteiro em PT e mensagem
  do `pingDB` copiada no `TestPartialFailure`.
- ✅ **1.3.3 fechada (D12, parte 1).** `processSource` devolve erro quando `failed/fetched` passa de
  0.5 (`fetched == 0` é sucesso, checado **antes** da divisão — com `int` seria panic, com `float64`
  seria `NaN` e passaria calado). `collectAndStoreData` passou a devolver só a contagem; o `if`
  morto e o bloco comentado saíram. `TestLimiarValidation` table-driven com JSON inline da Remotive
  (`{"id":1}` válido, `{"id":"x"}` inválido): todos inválidos → erro; 1 em 4 → `nil`; vazio → `nil`,
  afirmando retorno, `fetched`/`failed` e `runs.error`. Mutações em sentidos opostos: limiar 0 →
  pega no "1 em 4"; limiar 1.0 → pega no "todos inválidos".
- 🧠 **Pegadinha da divisão inteira, no lugar inesperado:** `const limiar float64 = 1 / 2` vale **0**
  — a conta é feita entre constantes inteiras e só o resultado vira `float64` (a spec traz
  `const Θ float64 = 3/2 // Θ == 1.0`). A divisão em runtime estava certa; a da constante, não.
- 🧠 Três bugs de teste em cascata, cada um escondendo o seguinte: handler que não escrevia o body;
  `err` do `run` sobrescrito pelo `Scan` (→ `err.Error()` sobre `nil` = panic, que derruba o
  **binário** inteiro, não só o subteste); `WHERE error IS NOT NULL` com `QueryRow` (casos de
  sucesso viram "no rows"). E `t.Fatalf` dentro de handler HTTP: `FailNow` só pode ser chamado
  da goroutine do teste — no handler, `t.Errorf`.
- 🟢 Mencionados uma vez: `limiar` e `TestLimiarValidation` em PT; subtestes `t1`/`t2`/`t3`;
  `itens` → `items`; `failedPct` guarda fração, não percentual.
- ✅ **1.3.4 fechada (D12 parte 2 + D11). Tarefa 1.3 fechada.** Migration `006` (`finished_at
  timestamptz`, nullable — run sem fim = processo morto no meio); `closeRun` grava `now()`. `defer`
  com `recover` declarado depois do `defer` do `closeRun`: LIFO faz ele rodar antes e preencher o
  `err` que o `closeRun` lê. `TestPanic`: tipo `dummy` satisfaz `Source` (interface no consumidor,
  D28, pagando de volta) e entra em panic no `Fetch`; `processSource` chamado direto com conexão de
  `newConn` (`t.Context()` para conectar, `context.Background()` no `t.Cleanup`, porque o contexto
  do teste é cancelado antes da limpeza). Mutações: sem `recover` → binário cai; sem `finished_at`
  → falha limpa; ordem dos `defer` invertida → falha, mas no `Scan` (`converting NULL to string`).
- 🧠 Typos na `006` (`timestampz`, `Dwon`) foram **barulhentos**: goose e Postgres resolvem nome em
  vez de filtrar — o lado bom da lição de 15/09. A `006` pôde ser editada porque o dev estava na 5.
- 🔴 **B6 — migration fora do commit.** O `feat` da 1.3.4 (`9ba1668`) foi para o `origin/main` com o
  `closeRun` gravando `finished_at`, mas sem a `006` (arquivo novo, `??`, não entrou no `git add`).
  Num clone limpo, o `UPDATE` do `closeRun` falharia em toda run. Resolvido em `cfca783`. Regra:
  `git status` antes de todo commit — "funciona na minha máquina" de novo (30/09).

### 2026-10-06 — Subtarefa 1.4.1 (retry com backoff)

- ✅ **1.4.1 fechada.** `HTTPGetBody` com laço de tentativas e `switch` por status: 200 → lê; 429/5xx
  → fecha o body, e se não for a última tentativa espera num `select` entre `time.NewTimer` e
  `ctx.Done()` (dobrando a espera); outro status → erro na hora. `ctx` vencido devolve `ctx.Err()` com
  `%w`. `NewClient(retries, timeout, initialTime)`; o `run` recebe espera/tentativas, então os testes
  do `main` usam 10ms (`TestPartialFailure`, que faz retry contra 500, em 0.11s). Testes em
  `internal/client`: `TestGoodServer`, `TestClientRetries` (4 casos, com contagem de requisições),
  `TestIncTime` (intervalos ≥ 10/20/40ms + exatamente 4 requisições), `TestCtxCancel` (`errors.Is`
  + 1 requisição + duração). Bateria de 5 mutações, todas pegas.
- Feito pelo mentor a pedido: chamadas do `run` no `main_test.go` + parâmetros em minúsculo
  (mecânico; o arquivo tinha um localizar/substituir quebrado: `(slog.Newslog...)`, `run(;`), e a
  medição de duração + `Close` no `TestCtxCancel` (o aluno insistiu).
- 🧠 Três testes passaram pelo motivo errado, cada um pego só por mutação: (1) panic no handler
  (índice fora da lista) vira **erro de rede** no client — o `net/http` recupera o panic e fecha a
  conexão —, e o `wantErr` ficava satisfeito; (2) asserção de intervalos sobre uma lista com 1
  instante não checa nada ("zero concorda com zero", 14/09); (3) client que ignora o `ctx` na espera
  ainda devolve `DeadlineExceeded` — na requisição **seguinte** —, só que 1s depois; só o tempo
  distingue.
- O handler do `TestClientRetries` travou 4×; resolvido trocando o **dado** do caso (404 seguido de
  200: client com bug tem sucesso) em vez da lógica do handler. Guarda da linha 55 ficou morta (🟢).
- 🟢 Mencionados uma vez: `Retries` conta tentativas, não retries; `trieDuration`/"trie"; `t.Stop()`
  depois do timer disparar é inócuo; prefixo `context canceled:` também em prazo vencido; erro do
  `default` sem URL; `TestIncTime`/`initialTim`/`t1..t4`; `10` repetido no `TestIncTime`.

### 2026-10-07 — Subtarefa 1.4.2 (paginação do Himalayas)

- ✅ **1.4.2 fechada.** `Fetch` em laço sobre `range` do teto (`Ceil`, campo do `Himalayas`, 5 no
  `For`); próxima URL com `url.Parse` + `Query().Set("cursor")` + `Encode`; para quando o
  `nextCursor` vem vazio — `break` **depois** do `append`, para não perder a última página. Falha em
  qualquer página → `fetch page N: %w` (e `... in page N` na decodificação), nenhum item devolvido.
  Testes em `internal/sources` com servidor-roteiro (cursor recebido → resposta em raw string):
  `TestCursor` (3 páginas, 6 itens, 3 requisições), `TestHighCeil` (sempre há cursor, teto 4),
  `TestMiddleErr` (página 2 com 404 → erro com "page 2", 0 itens, 2 requisições — prova também que
  404 não reintenta). Mutações sem teto / sem página no erro / sem parar no cursor vazio: todas pegas.
  `go run .` → run #52: Himalayas `fetched=100`, `failed=0`, 2.5s. `TestFetch` de 17s (teto 500) para 0.5s.
- Refactor do aluno no client (`709f5b7`): body lido e fechado uma vez antes do `switch`; `continue`
  na última tentativa no lugar do `break` — aplicou a resposta da pergunta da 1.4.1.
- 🧠 O cursor é **opaco**: o client devolve sem interpretar, então o servidor falso usa `"p2"`/`"p3"`.
  O aluno começou simulando um cursor de verdade (base64 de offset) — desnecessário.
- 🧠 O primeiro `TestCursor` pegou um bug real: 200 itens em vez de 6 (sem parada no cursor vazio,
  a URL mantinha o último cursor e repetia a página 3 até o teto de 100).
- Feito pelo mentor a pedido: renomear `url`/`URL` → `u`/`rawURL` (o pacote `net/url` ficava
  sombreado) e a higiene dos handlers (`t.Errorf`, `default` com status 400, `defer srv.Close()`).
- 🟢 Mencionados uma vez: `Ceil` (nome de função matemática; `MaxPages` dizia melhor), `make` com
  capacidade `20 * Ceil`, "fetchs", servidores de teste quase idênticos copiados 3×.

### 2026-10-07 — Subtarefa 1.4.3 (config por fonte)

- ✅ **1.4.3 fechada.** `SourceConfig` ganhou `timeout`, `retries`, `initial_wait` (durações com
  unidade, `time.Duration` direto do `yaml.v3`). **Desvio aceito do cartão:** o aluno fez do `timeout`
  o **orçamento da fonte** (antigo `CollectionTimeOut`, removido), não o timeout HTTP (segue 30s fixo
  no `run`); Himalayas com 120s porque pagina. `run(logger, dbURL, cfg)` sem parâmetros de retry.
  `Load` valida só fontes ligadas, `<= 0` → `config <fonte> <campo> must not be 0`.
- Verificação ao vivo feita pelo mentor numa cópia com `DATABASE_URL` falso (o aluno disse ter
  rodado; nenhuma run nova apareceu após a #52): sem `retries` na remotive → `exit=1` com `config
  remotive retries must not be 0`; config correto → passa do `Load` e só falha na conexão.
- Pelo caminho: tags `yaml:"url"` repetidas → `Load` em **panic** na partida (`duplicated key`);
  campos `int` com conversão `* int(time.Second)`; depois `Duration * time.Second` → **"segundos ao
  quadrado"** (5s → ~158 anos; 100ms → ~3 anos); tag `initial_time` × YAML `initial_wait` pega pelo
  `KnownFields` (o D31 trabalhando); `)` sobrando; `TestPanic` com `cfg.Timeout` = 0 → contexto
  nasce vencido e falha no `getSourceID` antes do `Fetch` — o D38 em pessoa, num `Config` que não
  passou pelo `Load` (a pegadinha do cartão).
- Feito pelo mentor a pedido: `Config` dos testes do `main` (`testTimeout`/`testRetries`), renomear
  `InitialTime` → `InitialWait` / `initial_time` → `initial_wait` em todo o código, e o `TestLoad`
  (o aluno insistiu): `want` é o `SourceConfig` inteiro comparado com `!=`, YAML na coluna 0,
  `errContains` com fonte e campo. Mutações: validar desligadas / sem checar `retries` / erro sem
  nome da fonte — todas pegas.
- 🟢 Mencionados uma vez: mensagem "must not be 0" para uma checagem `<= 0`; `timeout` como nome de
  orçamento (sugerido `budget`).

### 2026-10-07 — Subtarefa 1.4.4 (rate limit) · Tarefa 1.4 fechada

- ✅ **1.4.4 fechada.** `Client` guarda `LastRequest` (fim da última requisição, depois de fechar o
  body); no topo de **cada volta** do laço de tentativas espera só o que falta (`time.Until(last +
  interval) > 0`), com `select` timer × `ctx.Done()`. Um client por fonte → limite por fonte. Depois
  de um backoff maior que o intervalo, a conta dá negativa e o rate limit não soma espera.
  `interval` (1s) no `config.yaml` + validação no `Load`. `TestInterval` (servidor 200, 3 chamadas,
  `len == 3`, intervalos ≥ 50ms) e `TestIntervalCtx` (intervalo 1s, 2ª chamada com `ctx` 50ms →
  `DeadlineExceeded`, < 500ms, 1 requisição). Mutações: sem espera / sinal invertido / sem registrar
  o fim → `TestInterval`; espera ignorando o `ctx` → `TestIntervalCtx`. Run #55: Himalayas 6.3s.
- O aluno questionou a espera dentro do `for` ("isso é o retry, não uma nova requisição"): a primeira
  volta **é** a requisição nova; o estado vive no `Client` e atravessa chamadas. Fora do `for`, retry
  com `initial_wait` < intervalo furaria o limite. Ele moveu para dentro.
- Pelo caminho: `LastRequest` nunca preenchido + sinal invertido (`Now().Sub(last+interval)`,
  negativo → timer dispara na hora) — dois bugs, um escondendo o outro; depois o `if` corrigido com
  `time.Until` mas o timer ainda com o sinal antigo; `defer t.Stop()` dentro do `for`. Limite do
  `TestIntervalCtx` no intervalo inteiro pegava a mutação por **0.9ms** (1.0009s × 1s) — subiu para
  metade do intervalo.
- 🧠 Duração entre dois instantes: "o que falta" é `alvo − agora` (`time.Until`), não `agora − alvo`.
- Feito pelo mentor a pedido: 4º argumento nas chamadas de `NewClient` dos testes (`noInterval`),
  `interval` no `TestLoad` (+ caso "sem interval") e a execução ao vivo (run #53–#55).
- 🟢 Mencionados uma vez: `time.Until` calculado 2× (variável); `select` com timer duplicado (helper
  "espera respeitando ctx"); `50ms` repetido no `TestInterval`; chamadas repetidas (`for range 3`);
  `LastRequest` exportado; mensagem "less than 1 second" com limite de 500ms; `begining`/`expeceted`.
- 🟡 **D18 reaberto (resto):** `insertRawJobs` (`main.go:439`) ainda loga `Info` por vaga nova —
  ~200 linhas por run; vem do primeiro commit e escapou do 0.5.1.

### 2026-10-07 — Limpeza geral (mentor, a pedido do aluno) e reordenação da Fase 1

- **Medição para a 1.5:** 0 duplicatas entre fontes em 607 vagas; 6 empresas em 2+ fontes, todas com
  vagas diferentes. `posted_at` vazio em todas (D33). Fase 1 reordenada: 1.6 → 1.5 → 1.7.
- **Organização:** `main.go` (467 linhas) dividido **dentro do pacote `main`**, sem mudar lógica:
  `main.go` (orquestração: `run`, `processSource`, `collectAndStoreData`, `newSource`),
  `config.go` (`Config`, `loadConfig`), `migrate.go` (`embed` + goose), `store.go` (SQL e
  `contentHash`); `TestLoad` foi para `config_test.go`. Pacotes próprios (`internal/store`,
  `internal/config`, SPEC §7) ficam para quando um segundo binário (`radar`, 3b) precisar deles.
- **Nomes:** `limiar` → `failureThreshold`; `Ceil` → `himalayasMaxPages` / campo `MaxPages`;
  `For` → `newSource`; `Load` → `loadConfig` (exportar em `package main` não serve a ninguém);
  `compactJSON` → `contentHash` (devolve um hash); `insertRawJobs` → `insertRawJob`; `rawjobID`;
  `LastRequest` → `lastRequest` (estado interno); variável `sql` → `query` (sombreava o pacote
  `database/sql`, mesma classe do `url` da 1.4.2); `migrationsFS` local → `sub`; `envelop` →
  `envelope`; 30s mágico → `httpTimeout`.
- **Client:** helper `sleep(ctx, d)` usado pelo backoff e pelo rate limit (fim do `select`
  duplicado e do `time.Until` calculado 2×); body fechado também quando a leitura falha (antes
  vazava); erros `waiting to retry` / `waiting for rate limit` (antes `context canceled:` mesmo em
  prazo vencido), `unexpected status N from URL`, `giving up after N attempts: last status S`
  (fecha a parte "último status" do D38).
- **Comentários desatualizados corrigidos:** `collectAndStoreData` ainda falava em transação
  (removida no D29); `insertRawJobs` falava em "cada item" e "failed count".
- **Testes:** nomes descritivos (`TestFailureThreshold`, `TestBackoffGrows`, `TestBackoffHonorsCtx`,
  `TestIntervalHonorsCtx`, `TestPageCap`, `TestPageFailure`, subtestes legíveis); helpers
  `arrivals`/`assertGaps` (client) e `pagedServer` (Himalayas, antes copiado 3×); handler do
  `TestClientRetries` conta antes de indexar (a guarda morta saiu); bug no `TestPanic` (segunda
  mensagem imprimia `err` em vez de `runsErr`); `%v` → `%w` no `setupPostgres`; erros de `Write`/
  `Close` tratados; typos (`succeded`, `containg`, `itens`, `rmeove`, `trie`, `fetchs`…); comentário
  em PT com espaço não separável no `TestPartialFailure`.
- **Analyzer:** mensagens e nomes em inglês; `description` real no `pyproject.toml`. `ruff`, `ruff
  format`, `mypy --strict` limpos. O `EXE002` local é artefato do disco `fuseblk` (todo arquivo
  aparece executável; no git é `100644`, `core.fileMode=false`).
- **Verificação:** `gofmt`/`go vet` limpos; `golangci-lint` de 8 avisos → **0**; suíte verde com
  `-race`; 15 mutações (as das subtarefas 1.4.x + config) todas pegas. A mutação "uma tentativa a
  mais" agora é pega por **dois** testes (antes só pelo de backoff). Não coberto (nem antes): esperar
  o backoff depois da **última** tentativa.
- 🟡 D18 pago de vez (`new raw_job inserted` → `Debug`).

### 2026-10-07 — Subtarefa 1.6.1 (laço de coleta)

- ✅ **1.6.1 fechada.** `collect_every` no `Config` (validado `> 0` no `loadConfig`, `6h` no
  `config.yaml`) + caso "without collect_every" no `TestLoad`. `collector(ctx, logger, interval,
  cycle)`: ciclo na hora, depois um `time.Ticker` (criado uma vez, `defer t.Stop()`) num `for` +
  `select` entre o tick e o `ctx.Done()`; ciclo que falha vira `ERROR` e o laço segue; `ctx`
  cancelado → `INFO shutting down` e retorno. No `main`: `signal.NotifyContext(SIGINT, SIGTERM)`,
  flag `-once` (passada única, sai 1 se falhar) e o `run` fechado numa closure — o `collector` decide
  **quando**, o `main` decide **o quê**.
- Testes sem banco: `TestCollector` (tabela: "runs until cancelled", "keeps going after failed
  cycles") e `TestFirstCycle` (intervalo 1s, volta em < 500ms). Mutações: `select` sem `for` → os
  dois casos da tabela; sem a chamada imediata → `TestFirstCycle` (1.0s); `return` depois do erro na
  chamada imediata → `got 1`; no tick → `got 2`.
- Ao vivo: runs #56–#58 (`go run .` + Ctrl+C) e #59–#61 (`-once`), 0 falhas, Himalayas ~6s.
- Pelo caminho: ticker recriado a cada tick e `defer` dentro do `for`; `ctx.Done()` só logava e o
  laço girava para sempre (o teste travou — era o teste funcionando); ciclo falso que devolvia o erro
  **antes** de cancelar → `t2` pendurado; corrigido para "cancela na 1ª, espera 1", que passava com
  as duas mutações de `return` (não provava nada); TAB na indentação do YAML do caso novo.
- Feito pelo mentor a pedido: a última rodada de pendências (ordem cancelar → falhar e linha (b) com
  3/3, TABs do YAML, `WithCancel` + `defer`, QF1011, nomes dos casos, comentários do `main` e do
  `collector`).
- ⚠️ Para a 1.6.2: o ciclo não recebe o `ctx`, então Ctrl+C **durante** uma coleta só encerra quando
  ela termina (hoje ~8s; pior caso = soma dos orçamentos das fontes). O `docker stop` dá 10s antes do
  SIGKILL → run sem `finished_at`. Decidir entre `stop_grace_period` e passar o `ctx` ao ciclo.
- 🟢 Mencionados uma vez: `flag.Parse()` no meio do `main`; mensagem `bidibi bodobo` no ciclo falso.

---

### 2026-10-08 — Subtarefa 1.6.2 (parada limpa e compose de daemon)

- ✅ **1.6.2 fechada.** `run(ctx, …)` e `processSource(ctx, …)`: os três prazos (banco 5s,
  `openRun` 10s, orçamento da fonte → `collectionCtx`) **derivam** do `ctx` do sinal; o `closeRun`
  continua com `Background` (grava o fim justamente depois do cancelamento). O laço de fontes
  confere o `ctx` no topo de cada volta e devolve `ctx.Err()` embrulhado. No `main`, a closure
  captura o `ctx` (o ciclo não mudou de forma — caminho do aluno, equivalente ao do cartão).
- `TestCtxCancel`: servidor que só responde quando a requisição é cancelada, duas fontes com
  orçamento de 2s, `cancel` aos 100ms → `run` volta em < 1s com `context.Canceled`, o erro não cita
  `remotive`, 1 run com `finished_at` e `context canceled`. Mutações: orçamento nascendo do
  `Background` → 2.01s; sem a parada do laço → erro cita `remotive`.
- `compose.yaml`: `analyzer` depende do `postgres` saudável (antes esperava o `app` terminar — com
  daemon, nunca); `app` com `restart: unless-stopped`, `./config.yaml:/app/config.yaml:ro` por cima
  da cópia da imagem, `json-file` com `max-size "10m"` / `max-file "3"`. `go mod tidy`: `yaml` deixa
  de ser `// indirect`.
- Ao vivo: `docker compose stop app` no meio da coleta em **0.33s**; run #70 (Himalayas) fechada com
  `fetch page 2: waiting for rate limit: context canceled` — o cancelamento chegou dentro do `sleep`
  com `ctx` da 1.4.4. `collect_every: 1m` sem rebuild: coletas às 14:53:36, 14:54:42, 14:55:41.
  Passo 5 rodado pelo mentor (não havia rastro no banco); o `up -d --build` do passo 4 também.
- Pelo caminho: os `WithTimeout` **apagados** em vez de derivados (`TestClientTimeout` caiu: sem
  orçamento); a parada do laço lendo o `err` da função (sempre `nil` desde o `migrate`, porque o
  `source, err :=` do laço cria outro `err` a cada volta) — conferência morta, idêntica a não ter;
  depois embrulhando `err` nil → `%!w(<nil>)`. Erro do mentor: o cartão previa que sem a parada
  apareceriam 2 runs, mas com o `openRun` derivado a 2ª fonte falha antes do `INSERT` — corrigido
  para "o erro não cita a 2ª fonte".
- Feito pelo mentor a pedido: a conferência "não cita `remotive`" e a limpeza dos comentários e
  mensagens copiados do `TestClientTimeout`.
- 🟢 Mencionados uma vez: a parada confere só `context.Canceled` (não `Err() != nil`); cancelamento
  durante a **última** fonte, com as anteriores bem-sucedidas, faz o `run` devolver `nil` (`-once` +
  Ctrl+C nesse momento sai 0 — a run fica registrada com o erro); `processSource` recebe `timeout` e
  `sourceConfig.Timeout` (o mesmo valor duas vezes).

---

### 2026-10-08 — Subtarefa 1.6.3 (deploy no Pi) e escolha de valores

- **Medição para escolher ciclo e páginas** (banco do PC): RemoteOK e Remotive trazem **0–1** vagas
  novas por coleta mesmo depois de 13h; o Himalayas traz **100 de 100** novas sempre — o feed tem
  118.068 vagas, ordenado por `pubDate`, e 100 vagas cobrem só 17–78 min de publicações. A Remotive
  orienta no máximo 4 coletas por dia (README da API). Decisão: `collect_every` fica em **6h**;
  `himalayasMaxPages` 5 → **20** (20 × 1.5s + 60s de pior caso = 90s ≤ orçamento de 120s); log fica
  em 10m×3 (~3 KB/dia). Curiosidade: a 1ª página do Himalayas às vezes vem atrasada (às 11:48 a
  vaga mais nova era das 09:34; às 11:52, das 11:33) — cache do lado deles, provavelmente.
- ✅ **1.6.3 feita.** Pi: Docker `enabled`, chave SSH da conta (o mentor recomendou senha na chave;
  deploy key só se houver deploy automático), clone em `~/pi/job-searcher`, `.env` próprio. Banco
  do Pi é novo: runs #1–#3 (RemoteOK 99, Remotive 19, Himalayas **400 em 23.5s** — previsto 25–30s).
- **Reboot trocado por `sudo systemctl restart docker`** (o aluno não quis reiniciar o Pi): mesma
  prova das políticas de restart, sem o compose. Resultado: `shutting down` no SIGTERM, containers
  de volta sozinhos (`app` Up, `postgres` healthy) — mas o 1º ciclo falhou: `failed to open db
  conn: … dial tcp 172.18.0.3:5432: connect: connection refused`. No boot o Docker religa tudo ao
  mesmo tempo (o `depends_on` só vale no `compose up`); o `app` sobe em milissegundos, o Postgres
  leva segundos, e o próximo ciclo só vem em 6h. Vira a **1.6.4**.
- 🟢 Commit `88cdde0` com prefixo `refactor:` numa mudança de comportamento (devia ser `feat:` ou
  `chore:`); já estava no GitHub, ficou.

### 2026-10-08 — Subtarefa 1.6.4 (esperar o banco na partida)

- **1ª versão (🔁):** `Ticker` com período fixo (dobrar a variável não mudava o intervalo); prazo de
  2s num `dbCtx` que o `select` não escutava — depois de 2s toda tentativa falhava na hora e o laço
  só pararia com SIGTERM (sonda numa cópia: **50 WARNs em 5s**, todos a 100ms); sucesso na 1ª
  tentativa não voltava e abria uma 2ª conexão; nenhuma conexão fechada.
- **2ª versão (🔁):** prazo no `main`, tentativa única dentro do laço — mas sem dobrar a espera,
  prazo em 2s em vez de 2 min, `Close` faltando, `flag.Parse()` depois da espera (`-h` esperaria o
  banco).
- **3ª versão (🔁):** `conn.Close(ctx)` **antes** de checar o erro → `nil pointer dereference` na 1ª
  tentativa falha (sonda: panic em `main.go:111`). Justo no caso para o qual a função existe.
- **4ª versão:** porta sem ninguém com prazo de 5s → **6 WARNs**, `DeadlineExceeded` em 5.0s; banco
  local de pé → `nil` em **3ms**, `pg_stat_activity` com **0** conexões sobrando. O aluno exportou o
  `sleep` do client (`client.Sleep`) em vez de copiar — aceito: uma espera com `ctx`, um lugar só.
- **Teste:** `TestWaitForDB` em tabela (banco do teste / `localhost:1` com 300ms). O aluno pulou os
  limites de tempo (3c) — o mentor concordou: o `errors.Is(err, context.DeadlineExceeded)` já
  separa "tentou até o prazo" de "desistiu cedo" (mutação `return err` depois do WARN: o caso da
  porta sem ninguém falha em 0.00s). Uma linha solta `main_tes` no `TestCtxCancel` quebrava a
  compilação; o aluno tirou. Nomes dos casos (`t1`/`t2` → `database up`/`nobody listening`) e o
  `$` sobrando no DSN trocados pelo mentor, a pedido.
- **Pi:** o aluno disse que tinha feito antes de commitar — `origin/main` ainda em `88cdde0`, então
  o Pi rodou o código antigo (se as runs vieram, foi sorte na corrida). Depois: `f3de19f` +
  `1b15b5c` no GitHub; o aluno relata o Pi certo, sem colar a saída. A prova com saída fica para
  o redeploy da 1.6.6.
- ✅ **1.6.4 fechada.** Sobrou um `errcheck` no `golangci-lint` (`conn.Close(ctx)` sem tratar o
  erro em `main.go:120`) — corrigir junto da 1.6.5.

### 2026-10-08 — Subtarefa 1.6.5 (saúde por fonte, D36)

- **1ª versão da função (🔁):** argumentos da consulta trocados (`$1` recebia o K) — no banco local
  os ids são 9/10/11, a consulta buscava `source_id = 2`, voltava vazia e as **3 fontes saudáveis
  saíam quebradas** (sonda numa cópia); com a ordem certa, `cannot scan NULL into *string` (o
  `error` das runs boas é NULL); regra pela **soma** de `fetched` (fonte sem runs = quebrada; run
  com erro e itens + run vazia = não quebrada); erro devolvido com `true`.
- 🔴 **Mudança acidental no `insertRawJob`**: o `existed` virou `true` em toda saída — toda vaga
  nova era pulada antes do `insertJob` e `jobs` parava de crescer sem erro. O `TestFetch` pegou
  (`expected 137; got 0`). Na 2ª edição ficou **invertido** (duplicata `false`, vaga nova `true`).
  Nada foi commitado nem chegou ao Pi.
- **2ª versão:** o aluno acertou a ordem, o NULL (`error IS NOT NULL` no SQL, já vem bool),
  `rows.Err()` e tirou o `logger`. O resto foi corrigido pelo mentor, a pedido: `insertRawJob` de
  volta ao original, regra por run (`runs == k && failed == k`), erros com `false` e `%w`, nomes
  (`sourceLooksBroken`, `brokenAfterRuns`) e o `errcheck` da 1.6.4.
- **Teste** (`TestSourceLooksBroken`) escrito pelo mentor, a pedido: 5 casos, `TRUNCATE runs` por
  caso, como os outros testes. A run com erro **e** 100 itens (o `failureThreshold` fecha assim)
  cobre a regra antiga da soma. Mutações não rodaram (o aluno interrompeu).
- **Ligação no `run`** pelo aluno: segundo laço depois do das fontes, antes do `if !success`; pula
  desligadas; ctx de 10s por consulta; erro da consulta só vira log. Ao vivo numa cópia (remoteok e
  himalayas desligadas, remotive em `https://remotive.invalid`): 1ª passada sem aviso, 2ª com
  `WARN source is broken source=remotive after=2` mesmo com o ciclo falhando. `golangci-lint` com
  0 issues, suíte verde com `-race`.
- 🟢 `defer cancel()` dentro do laço: os cancelamentos se acumulam até o `run` voltar (3 timers).
- ✅ **1.6.5 fechada.** D36 fechado: zero itens em K runs agora aparece no log.

### 2026-10-08 — Subtarefa 1.6.6 (os 7 dias) — relógio começou

- `fa638fd` no GitHub; o aluno relata deploy no Pi e restart do Docker. **A saída não foi colada**
  (pedida 3×): a prova da 1.6.4 (voltar sozinho e coletar depois do restart) segue sem evidência
  direta. O dia 7 cobre em parte: se houver queda no meio da semana e a espera falhar, o maior
  intervalo passa de 6h.
- `db/queries/health.sql` (`de94bb3`), feito pelo aluno no formato "o que a consulta devolve +
  saída esperada" (ele pediu: o passo a passo em sub-passos não ajudou). 1ª consulta: runs e erros
  por dia e por fonte — certa depois de trocar `dia` → `day` no `GROUP BY`/`ORDER BY`; converte
  para São Paulo **antes** do `date_trunc` (a sessão do Postgres é UTC; a run #56, 01:24 UTC de
  08/10, cai em 07/10). 2ª: maior intervalo por fonte com `lag` numa CTE e `max` por fora — o
  filtro de `:'start'` faltava (remoteok com 14 dias); entrou **dentro** da CTE, antes do `lag`,
  para não medir o intervalo entre o aquecimento e o 1º ciclo. Ordem por maior intervalo, escolha
  do aluno (melhor que a pedida). Local: 13h22 nas 3 fontes (PC desligado de noite).
- Formato do `start`: `'2026-10-08 18:30:12-03'`, o `started_at` da consulta com `+00`, ou
  `date -Iseconds` — os três testados.

### 2026-10-16 — Subtarefa 1.6.6 fechada (conferência do dia 7) · Tarefa 1.6 fechada

- Conferência feita pelo mentor via `ssh pi` (só leitura), depois que o aluno não entendeu o passo
  de achar o início do relógio. **Antes:** o aluno tinha rodado `docker compose down` no Pi (journal:
  containers parados às 16:40:16) — volume intacto, mas **os logs da semana foram junto** (o log
  `json-file` mora no container), então o `grep` dos `WARN` virou leitura do banco. Subiu de novo
  com `up -d`.
- **O banco do Pi foi recriado no deploy da 1.6.5**: a run #1 é de 2026-10-08 17:56:46 -03 (as runs
  do aquecimento, 12:48, sumiram — provavelmente um `down -v`). Não houve o par de ciclos "deploy +
  restart um minuto depois": a prova do restart da 1.6.4 não foi feita no deploy.
- **Resultado** (`start = 2026-10-08 20:56:46+00`): 4 runs por fonte por dia (2 no dia parcial de
  08/10, 5 em 14/10); maior intervalo **6h03** nas 3 fontes. 105 runs, 9 com erro, todas por DNS.
- **Três quedas de energia, e a prova real da 1.6.4:** 14/10 ~16:12 e ~16:19 e 15/10 14:30 (o
  único boot no journal, que não é persistente). Em cada uma o compose voltou sozinho e o `app`
  conectou no banco (abriu as runs), mas o ciclo imediato falhou nas 3 fontes com `dial tcp: lookup
  remoteok.com on 127.0.0.11:53: server misbehaving` — o DNS embutido do Docker sem upstream: o
  roteador volta depois do Pi. Próxima coleta boa só 6h depois (D39). Às 16:19 de 14/10 a 2ª falha
  seguida teria disparado o `WARN source is broken` nas 3 fontes ao mesmo tempo — a assinatura de
  rede, não de formato (era a pergunta da 1.6.5).
- O `id` das runs pulou de 75 para 106 em 14/10: depois de um desligamento sujo, o Postgres
  recupera a sequência pulando os valores que já tinha pré-registrado no WAL (até 32). 135 ids,
  105 runs.
- ✅ **1.6.6 fechada; Tarefa 1.6 fechada.**
- **Cópia para o PC:** o aluno apagou o volume local; `pg_dump -Fc` do Pi (45 MB) restaurado no
  `jobs` local (`dropdb --force` + `createdb` + `pg_restore --no-owner`). 1ª tentativa deu 3 runs:
  o dump não tinha sido feito e um `docker compose up -d` completo subiu o `app` local, que migrou
  e coletou no banco vazio. O dump ficou em `db_dump/` dentro do repositório → `db_dump/` no
  `.gitignore`. Aula de `-T` (sem TTY quando há `<`/`>`/`|`) e dos binários do Postgres.
- **Medição da 1.5 (mentor, para planejar):** 382 grupos repetidos dentro da mesma fonte (1.102
  vagas, Himalayas; 190 com descrição idêntica), 2 entre fontes. Tarefa 1.5 detalhada no ROADMAP.

### 2026-10-16 — Subtarefa 1.5.1 (medir duplicatas)

- O aluno pediu o enunciado no formato "o que a consulta devolve + saída esperada" e explicação de
  `regexp_replace` (padrão `[^a-z0-9]+`, `lower` por dentro), `GROUP BY`/`HAVING` (coluna solta
  não pode aparecer na linha do grupo) e `FILTER`.
- Furos pelo caminho: `HAVING` some do `WITH` (todo singleton virava grupo: 11.860);
  `COUNT(vaga)` contava grupos, não vagas (→ `SUM`); na 2ª consulta, agrupar pelo texto original
  dava 0 linhas ("Full-stack" × "Full stack") e `source_id` solto no `WITH` dava erro de
  agregação — resolvido com as chaves no `WITH` e `JOIN` de volta a `jobs` pelas expressões
  normalizadas. Nomes trocados para inglês pelo mentor, a pedido; `ORDER BY kind` adicionado.
- **Para a regra (mentor):** nos 192 grupos da mesma fonte com descrição diferente, salário e
  `remote_scope` não variam (o Himalayas não preenche `remote_scope`); 106 diferem em menos de
  200 caracteres. Diff de duas versões da Bjak "Engineering Manager": só o bloco Location muda —
  "hybrid role, 3 days per week" × "remote role, we hire globally".
- ✅ **1.5.1 fechada.**

### 2026-10-17 — Subtarefa 1.5.2 (a regra de dedupe em Go)

- O aluno propôs mudar o pipeline (Go só grava `raw_jobs`, Python normaliza e deduplica); revisto
  e mantido "por enquanto" — ver decisões. Testando a chave no banco, o mentor achou 13 títulos
  que normalizam para `""` (10 da Gate, em chinês) → regra nova: chave com parte vazia nunca é
  duplicata, 6º caso na tabela.
- O aluno pediu explicação do que cada função faz, do que é "receber uma vaga" e de onde vem a
  fonte (não está no `Job`: na 1.5.3 vem do `sourceID` do `collectAndStoreData` e da coluna
  `source_id`). Escolheu `sources.Job` + id da fonte como parâmetros.
- Furos no 1º `/check`: sem `ToLower` (a expressão apagava as **maiúsculas**: `Lemon.io` →
  `emonio`, `IBM` → `""`); a regra saía com `true` só pela chave igual e testava a descrição no
  ramo das chaves **diferentes** (Bjak dava `true`). Os dois consertados pelo aluno; nomes do
  retorno trocados, `findSameJob` → `isDuplicate`, `regexExp` → `nonKeyChars` e comentários de
  documentação feitos pelo mentor, a pedido.
- O mentor errou a lista de mutações (tirar o `ToLower` não derrubava caso nenhum: Huzzle e
  Lemon.io têm as mesmas maiúsculas dos dois lados) → 7º caso real, MWDN (empresa em capitais,
  5 clones idênticos). Casos da tabela montados pelo mentor, a pedido; o laço com `t.Run` é do
  aluno.
- ✅ **1.5.2 fechada.** Suíte inteira verde com `-race`.

### 2026-10-17 — Subtarefa 1.5.3 (marcar `duplicate_of` na gravação)

- Critério de aceite da 1.5 corrigido (pedia "nenhum grupo com mais de uma vaga viva", o que
  contradiz a Bjak); antiga 1.5.3 partida em gravação (1.5.3), histórico (1.5.4) e re-medir (1.5.5).
- O aluno pediu a `007` no formato de especificação; `Up` certo de primeira; `Down` com
  `DROP INDEX CONCURRENTLY` quebrava (goose roda cada migração numa transação) — consertado.
- Perguntas boas do aluno: por que não juntar todas as candidatas numa lista (o `duplicate_of`
  guarda um `id` só; a lista viraria "pegar a primeira") e por que gravar `NULL` se a coluna já é
  nula (o `insertJob` também roda no `UPDATE` de uma vaga editada — a marca velha precisa sair).
  Daí saiu o **D40** (original editada deixa as cópias apontando para ela).
- Furos pelo caminho: chaves fora do `DO UPDATE SET`; `EXLUCDE` (derrubava todo `INSERT`:
  `TestFetch` 0 de 137); `UPDATE` **sem `WHERE`** (reescreveria as 12 mil vagas); escrita através
  de ponteiro `nil`; parâmetros do `UPDATE` trocados (a original virava cópia); `rows` aberto no
  `break` (`conn busy`). Consertados pelo aluno; o mentor juntou os dois `UPDATE` num só (a pedido),
  devolveu o `defer rows.Close()` e pôs o `Close` antes do `Err`.
- `TestMarkDuplicates` escrito pelo mentor, a pedido. Mutações rodadas pelo mentor (o aluno
  autorizou de vez): `id <= $3` → toda vaga aponta para si; `ORDER BY id DESC` → r2 → h5. Tirar o
  `id < $3` de vez só gera erro de parâmetro (3 args, 2 `$`) — mutação ruim, trocada.
- ✅ **1.5.3 fechada.**

### 2026-10-18 — Subtarefa 1.5.4 (o histórico)

- Dúvida do aluno: onde guardar `id` e `source_id` se o `sources.Job` não tem esses campos. Ele
  criou o struct no pacote `sources` e sem descrição; foi para o `store.go` (o par vaga + ids do
  banco é coisa do store) com um `sources.Job` inteiro dentro — sem a descrição, nenhum clone da
  mesma fonte seria marcado.
- Furos pelo caminho: busca sem `LIMIT` (as 12 mil vagas, ~65 MB, de uma vez); o slice no lugar
  da variável do laço; `SET` duas vezes no `UPDATE`; `continue` no erro (laço infinito: a vaga
  com erro volta na busca seguinte); sem `return` depois do laço. Consertados pelo aluno.
- A pedido, o mentor renomeou (`backfillDedupe`, `storedJob`, `backfillTimeout`,
  `backfillBatchSize`, `cancelBackfill` — o `run` já tinha um `cancel`), escreveu as mensagens de
  log com `duration` (para medir no Pi) e fez a função devolver quantas terminou também no erro
  (padrão do `io.Writer`). Também escreveu o `TestBackfillDedupe`, com os helpers
  `writeDuplicateScenario` / `checkDuplicateMarks` extraídos do `TestMarkDuplicates`.
- Mutações (mentor): um lote só → 3 de 10, r1/r2 sem marca; `ORDER BY id DESC` → h2, h3, r1, r2
  sem marca; sem `updateDuplicate` → as mesmas 4 (o teste zera as marcas, senão essa passaria).
- Medição na cópia local do Pi, numa transação desfeita: 12.584 vagas em 2,7 s, 2º passe 0 em
  5 ms; 448 marcadas (446 mesma fonte, 2 entre fontes), 12.136 vivas, 0 corrente, 0 apontando
  para si; critério de aceite 0 grupos entre fontes, 0 grupos da mesma fonte com descrição
  repetida. No Pi deve demorar mais (cada `UPDATE` faz commit no disco).
- ✅ **1.5.4 fechada.**

### 2026-10-18 — Subtarefa 1.5.5 (deploy no Pi + consulta de aceite) · Tarefa 1.5 fechada

- Deploy do aluno (`git pull` + `up -d --build app`): migração `007` em 87 ms; histórico de
  **15.279 vagas em 1 min 20 s** no Pi (~30× a cópia local numa transação só — cada `UPDATE` faz
  commit no disco), dentro dos 5 min; 565 marcadas, 16 chaves vazias, 0 chave `NULL`. A coleta
  das 14:25 trouxe o mesmo conteúdo da das 13:43 (0 vagas novas): a marcação na gravação só roda
  no Pi a partir do ciclo seguinte.
- `db/queries/live_duplicates.sql` no formato de especificação. 1ª versão devolvia `249 | 0`:
  `COUNT(source_id)` conta linhas, não fontes (todo grupo virava "entre fontes"); `n_desc >
  n_rows` nunca é verdade (a coluna dava 0 com ou sem dedupe); chaves filtradas por `IS NOT NULL`
  em vez de vazias. Corrigida pelo aluno: **`0 | 0`**; sem o filtro de vivas (rodado pelo mentor),
  `3 | 292`.
- ✅ **1.5.5 fechada · Tarefa 1.5 fechada.** Tarefa 1.7 detalhada no ROADMAP (1.7.1 D37, 1.7.2
  D39, 1.7.3 README + D17).

### 2026-10-18 — Subtarefa 1.7.1 (D37, Remote OK com `[]`)

- O aluno quis fazer o resto da Fase 1 de uma vez, no passo a passo; o protocolo segue uma
  subtarefa por vez, com o cartão seguinte junto da aprovação.
- `TestRemoteOK` do aluno. 1ª versão passava **pelo motivo errado**: o corpo do caso "vazio" era
  `[{"}]`, JSON inválido — o `Fetch` falhava no `Unmarshal` e nunca chegava ao `items[1:]`. O ✔️
  do passo 1 (ver o `panic` antes do conserto) foi o que pegou.
- Conserto (conferir o tamanho antes de cortar) feito pelo mentor, a pedido, junto com as duas
  sugestões opcionais do teste (`"legal"` no lugar de `"cool"`, `t.Cleanup(srv.Close)`).
- ✅ **1.7.1 fechada · D37 pago.**

### 2026-10-18 — Subtarefa 1.7.2 (D39, nova tentativa depois de ciclo falho)

- Helper do aluno usado nos dois lugares certos (partida e tick), com contador e `ctx`. Furos: a
  espera vinha com um ticker, então **toda** chamada esperava antes de rodar — a coleta da
  partida atrasaria 5 min mesmo sem falha (`TestFirstCycle` 1.0s); espera e teto fixos dentro
  do `collector`; constantes entre o comentário do `himalayasMaxPages` e a declaração (2ª vez);
  `Error` a cada falha; "shutting down" duplicado. Numa 2ª versão, `time.Sleep` sem o `ctx`.
- A pedido, o mentor terminou: `runWithRetries` ("chama → falhou? espera → chama"),
  `cycleRetries = 3` (novas tentativas, não chamadas) e `cycleRetryWait = 5m`, `Warn` por
  tentativa e `Error` ao desistir, saída silenciosa quando o ciclo falhou por desligamento.
  `TestCollectorRetry` com 3 casos; testes antigos passam `0, 0` (sem novas tentativas).
- Mutações (mentor): sem teto → 20 chamadas; sem nova tentativa → 1; `>=` no teto → 3 de 4;
  `Sleep` sem o `ctx` → teste estoura o prazo; espera antes da 1ª chamada (a 1ª versão do aluno)
  → **passava**, porque o `TestFirstCycle` usava espera 0. Corrigido: espera de 1s, cai com
  `expected time < 500ms; got 1.0s`.
- D17 pago pelo mentor (boilerplate): `Makefile` monta a DSN do `.env` (`set -a; . ./.env`), sem
  senha nem caminho no arquivo.
- ✅ **1.7.2 fechada · D39 pago · D17 pago.**

### 2026-10-18 — Subtarefa 1.7.3 (README) · Tarefa 1.7 fechada · **Fase 1 fechada**

- README escrito pelo mentor, a pedido. O antigo descrevia a Fase 0: `app` saindo com 0,
  `config.yaml` embutido na imagem (hoje é montado `:ro` por cima), migrações até a 3.
- Conferido contra o código: `retries` = tentativas no total (`for i := range c.Retries`),
  `initial_wait` dobra a cada nova tentativa, limiar de itens `> 0.5`, `jobs.raw_job_id` aponta
  para a versão mais nova. `health.sql` (com `-v start=…`) e `make gstatus` rodados.
- Achado ao escrever o Quick start: o `analyzer` depende só do `postgres` saudável, não do `app`
  — virou o **D41**.
- ✅ **1.7.3 fechada · Tarefa 1.7 fechada · Fase 1 fechada.**

### 2026-10-18 — Auditoria de fim da Fase 1 (a pedido do aluno)

- Sem 🔴. Lint, `tidy`, testes e Python limpos; Pi saudável (143 MB, disco 41%).
- Docs: marcas de status velhas no ROADMAP e no PROGRESS corrigidas; três decisões revistas
  ganharam a nota de revisão. SPEC (aprovado pelo aluno): coletor com laço próprio (não cron),
  I/O hoje sequencial, ATS e Arbeitnow em backlog, estado atual do layout no §7.
- Repo (aprovado pelo aluno): `db/diagram/job_searcher.png` (31/08, anterior às migrações 003–007)
  removido; branches `feature/client` (mesclada), `backup/rascunho` e `origin/etapa-01` (rascunhos
  já superados; a `etapa-01` expunha o `CLAUDE.md`) a apagar pelo aluno.
- Nits pagos pelo mentor, a pedido: comentário do `run` (histórico + aviso de fonte quebrada),
  `defer cancel()` no laço de fonte quebrada virou `cancelCheck()` logo após a chamada (sugestão
  da 1.6.5), typo "Cluade" no `.gitignore`.

### 2026-10-18 — Abertura da Fase 2 (diagnóstico)

- Diagnóstico: nada bloqueando. Commits da auditoria, branches apagadas e `git pull` no Pi feitos
  pelo aluno; o `up -d --build app` não rodou (container e imagem de 14:24 UTC, `26cccd7`): D37 e
  D39 ainda não estão em produção.
- Medição no Pi para planejar: 15.279 vagas (14.714 vivas), **99% Himalayas**; Remote OK 137 e
  Remotive 19 no total — os feeds só devolvem as mais recentes. Descrições em HTML (100% da
  Himalayas), entidades em metade, `posted_at` vazio em todas.
- Fase 2 detalhada no ROADMAP (2.1 em subtarefas; 2.2–2.5 em resumo). Decisões de método: dados
  da EDA vêm do banco local restaurado do Pi; dependências de EDA num grupo `eda` (fora do `dev` e
  da imagem); notebook só para explorar, o que roda de verdade vai para `.py` com `pytest`.

### 2026-10-19 — Diagnóstico

- Nada bloqueando. O `up -d --build app` no Pi rodou (imagem e container de 2026-10-18 20:00 UTC,
  `8498ca2`); runs 163–171 limpas nas 3 fontes. Pi: 16.800 vagas, 16.204 vivas.

### 2026-10-19 — Subtarefa 2.1.1 (bancada de EDA)

- Dump do Pi restaurado no `jobs` local (`db_dump/jobs-2026-10-19.dump`): 16.800 vagas, 16.204 vivas.
- **Método revisto:** o aluno explora num REPL IPython dentro do nvim (`iron.nvim`, células `# %%`),
  não no Jupyter. Grupo `eda` = `pandas` + `ipython`; `analyzer/eda/01_overview.py` no lugar do
  notebook; `eda` no `.dockerignore`. `dotenv` tirado do grupo (o `--env-file` do `uv run` carrega o
  `.env`).
- O `iron.lua` do aluno (`~/.config/nvim/after/plugin/iron.lua`, fora do repo) abria o REPL sem o
  `.env` e sem o grupo `eda` (o `pandas` só aparecia por sobra no `.venv`). Ajustado pelo mentor, a
  pedido: `--all-groups` + `--env-file` (pasta do `pyproject` ou raiz do git). Sem commit no repo do
  nvim.
- Furos no caminho: `fetchone` imprimindo a tupla `(16800,)`; `return 1` copiado do `main.py` (no
  script ninguém lê o retorno: exit 0 e nenhum `DataFrame`) → exceção; query agregada (`COUNT` +
  `GROUP BY`) no lugar de uma linha por vaga; f-string sem o `f`; `I001` nos imports.
- Aula, a pedido: `fetchall`, `description`, list comprehension e `DataFrame(columns=)`, com
  exemplos de outro domínio.
- ✅ **2.1.1 fechada.** `(16204, 18)`: Himalayas 16.049, Remote OK 136, Remotive 19. Erro de
  configuração sai com exit 1. `ruff` limpo (só o `EXE002` do NTFS). Sugestão opcional em aberto:
  `mypy` acusa `pandas-stubs` ausente e `cur.description` opcional.
- **Pedido do aluno:** em Python, uma mini-aula antes de cada subtarefa: curta, direta, com
  exemplos genéricos de outro domínio. Virou o bloco **Antes de começar** no `CLAUDE.md` (§1, §4,
  §8); começa na 2.1.2.

### 2026-10-19 — Subtarefa 2.1.2 (primeiro retrato) · Tarefa 2.1 fechada

- O aluno pediu o cartão "mais objetivo": virou duas tabelas (os `DataFrame`s e as análises, cada
  uma com a saída esperada). Formato que funcionou.
- Do aluno: segunda query em `raw_jobs`, primeira coleta com `groupby` por chave composta +
  `as_index=False`, `describe` por fonte, top 15. Furos: o `merge` usava a tabela de **versões**
  (16.888 linhas, a pegadinha do cartão) com `validate="one_to_many"`, que deixa a multiplicação
  passar; a data virou texto (`strftime`) antes do `merge`; `import matplotlib as plt`.
- A pedido, o mentor fez o histograma, os comentários e a estrutura dos prints (`section`,
  `display.max_columns`). O aluno corrigiu o `merge` (`first_fetch`, `"one_to_one"`).
- Achado ao escrever o comentário das vagas por dia: a curva plana (~1,1–1,6 mil/dia) é o teto de
  20 páginas da Himalayas, não o mercado → **D42**.
- ✅ **2.1.2 fechada · Tarefa 2.1 fechada.** Tamanho mediano 5.073 / 4.146 / 4.980 (Himalayas /
  Remote OK / Remotive), máximo 36 mil; Veeva Systems 233; 12 dias somando 16.204.
- Medição para a 2.2.1 (mentor): HTML em 100% das descrições nas 3 fontes; entidade HTML em 86
  títulos, todos da Himalayas (`&#x2f;`, `&ndash;`); mojibake (caracteres `\x80`–`\x9f`) em 102 de 136
  descrições do Remote OK e 4 da Himalayas, as 102 reversíveis com `encode('latin-1')` +
  `decode('utf-8')`. Tarefa 2.2 detalhada no ROADMAP.

### 2026-10-20 — Subtarefa 2.2.1 (defeitos de texto)

- Diagnóstico: nada bloqueando; Tarefa 2.1 ainda sem commit.
- Do aluno: `eda/db.py` (`sql_to_dataframe`, SQL → `DataFrame`) usado nas duas queries do
  `01_overview.py`; máscaras de HTML e de entidade com fração por fonte; regex de entidade cobrindo
  nomeada, decimal e hexadecimal.
- Furos no caminho: `re.search` com os argumentos trocados (`PatternError: multiple repeat`); regex
  de HTML exigindo par abre/fecha sem atributo (0,772 no Remote OK) e depois `<.*>` guloso (match de
  4.116 de 4.403 caracteres, e casa `<5 years and >2`); máscara de entidade no `assign` do mojibake;
  regex de mojibake procurando o **apelido** do `repr` (`\\x8a`) e depois montando o escape com
  classes (`\x[8-9][a-f]`, `incomplete escape`).
- Aulas, a pedido: mojibake (bytes, tabela errada, as duas assinaturas, reversível) e "o `repr`
  mostra um apelido" (com o TAB). Travou três vezes no escape; a pedido, o mentor montou
  `[\x80-\x9f]` e explicou peça por peça (`[a-f]` é `[\x61-\x66]`).
- A pedido, o mentor fez o passo 4 e a arrumação: `measure_defect` (fração + primeiro exemplo com
  `id` e fonte), comentários, `ruff --fix` nos imports.
- ✅ **2.2.1 fechada.** HTML 100% nas 3 fontes; entidade em 86 títulos da Himalayas (`&#x2f;` 47,
  `&#x28;`/`&#x29;` 32, `&ndash;` 27); mojibake em 102 de 136 do Remote OK (todas reversíveis) e 4 da
  Himalayas (texto da mesma empresa, com Unicode real misturado: a volta inteira falha).
- Medição para a 2.2.2 (mentor): em `jobs`, `posted_at`, `remote_scope`, `salary_*` e `currency`
  estão vazios em 100% das vivas. Nos payloads: data em 100% nas 3 fontes; localização 97%
  (Himalayas), 57% (Remote OK, 58 `""`), 100% (Remotive); salário 41% (Himalayas, `null`), 18%
  (Remote OK, 112 `salary_min: 0`), 58% (Remotive, texto livre, 8 `""`). Himalayas mistura
  `salaryPeriod` anual, mensal e por hora.

### 2026-10-20 — Subtarefa 2.2.2 (campos descartados e sentinelas)

- Do aluno: query das vivas com o `payload` da versão mais nova (`jobs.raw_job_id`, conferido: as
  16.800 apontam para o `max(id)` do `raw_jobs`); `json_normalize` por fonte numa função; o `dict`
  fonte → colunas e o `for` pelo `dict` (passo 3); descoberta das sentinelas com
  `value_counts(dropna=False)`; passo 5 com `notna().groupby(<Series de fora>).mean()`.
- Travou no desenho "uma função para fontes com colunas diferentes": o mentor apontou o padrão
  table-driven do Go (a tabela guarda o que muda, o laço o que é igual) e que a sentinela depende do
  **tipo do valor**, não da fonte. Ficou perdido no `for` (copiava as três chamadas dentro do laço);
  passo 3 quebrado em três pedaços de 5–10 min.
- A pedido, o mentor fez o passo 4 (`is_informed` com a lista testada antes do `pd.isna`,
  `fill_rates` com `present`/`informed` lado a lado), a arrumação (`FIELDS`, `SENTINELS`,
  `source_payloads`) e os comentários. **Pedido do aluno: comentários e prints estéticos são sempre
  do mentor.**
- ✅ **2.2.2 fechada.** Informado de verdade: data 100% nas 3 fontes (epoch, ISO com fuso, ISO sem
  fuso); localização 97,3% Himalayas (432 `[]`), 57,4% Remote OK (58 `""`), 100% Remotive;
  salário 41,2% Himalayas (`null`, sem sentinela), 17,6% Remote OK (112 `0`), 57,9% Remotive
  (texto livre); tags 100% / 98,5% / 94,7%. Em `jobs`, as cinco colunas dão 0 nas 3 fontes (D33).
- Achados novos para a 2.2.3 / 3a: na Himalayas, `salaryPeriod` vem sempre (`annual` mesmo sem
  valor) e `currency` vem sozinha em 1.235 vagas, então só `minSalary`/`maxSalary` contam; os
  valores vão de 0,59 a 120M (período e moeda misturados). `categories` da Himalayas são slugs de
  cargo, não skills; as `tags` da Remotive são skills.

### 2026-10-20 — Subtarefa 2.2.3 (`docs/eda.md`) · Tarefa 2.2 fechada

- O aluno escolheu os 7 candidatos e pediu que o mentor escrevesse o documento ("já analisei
  tudo"). Cada defeito tem número com denominador, exemplo com `id`, dono e decisão.
- Medido para o documento: entidades nos títulos por título (`&#x2f;` 45, `&#x28;`/`&#x29;` 32,
  `&ndash;` 23); salário da Himalayas com valor em 6.617 vagas (5.186 anual, 1.001 hora, 425 mês,
  5 semana; 25 moedas, USD 83,7%; de 0,59 USD/hora a 168M IDR/ano); D42 na tabela `runs`: 47 runs
  terminadas da Himalayas, 44 leram as 400, em 37 delas ≥370 eram inéditas (mediana 398).
- Decisões: consertar no Python o HTML (2.3) e o salário heterogêneo (3a); consertar no Go as
  entidades do título e o mojibake (D10), os campos descartados e as sentinelas (D33, D32) e o
  teto de páginas (D42) — todos com dono na Tarefa 2.5, salvo os da 3a. Aceitos: os 4 mojibakes
  da Himalayas.
- Achado lateral (preparando a 2.3): 4 descrições do Remote OK (35, 70, 90, 15554) vêm com o HTML
  escapado duas vezes (`&lt;p&gt;`) e viram `<p>` literal depois do parse. Vai junto do D10.
- ✅ **2.2.3 fechada · Tarefa 2.2 fechada.** Tarefa 2.3 detalhada no ROADMAP (2.3.1–2.3.3).

### 2026-10-20 — Subtarefa 2.3.1 (HTML → parágrafos)

- Do aluno: subclasse de `HTMLParser` com `super().__init__()`, o `set` das tags de bloco, o
  "Enter" chamado só nas tags de bloco, o `close()` + último parágrafo, `pytest` com
  `parametrize`, o teste em `tests/`, `list[str]`.
- Travou duas vezes: no modelo "cada evento de texto é um parágrafo" (2 testes passavam por sorte)
  e no método com nome errado (`starttag`, que o parser nunca chama). Destravou com a analogia da
  carta (só o Enter muda de linha) e o passo quebrado em 4 pedaços.
- A pedido, o mentor fez: juntar antes de normalizar (`foo<b>bar</b>`), os 3 casos reais
  (Remote OK 5, Remotive 100, Himalayas 120, conferidos como prefixo exato da descrição no banco),
  `pytest.param` com `id` curto, os nomes (`html_to_paragraphs`, `_ParagraphCollector`,
  `_end_paragraph`, `BLOCK_TAGS`), o `close()` sobrescrito para fechar o último parágrafo, os
  comentários e o `[tool.pytest.ini_options]` que faltava.
- Mutações (mentor): sem o `h3` nas tags de bloco os casos reais passavam (o `<h3>` da 120 fica
  entre `</p>` e `<ul>`) → entrou `Intro<h3>Benefits</h3>Remote`; sem o último parágrafo no
  `close()`, 2 falham; com parágrafo vazio liberado, os 9 falham.
- ✅ **2.3.1 fechada.** `9 passed`, `mypy --strict` e `ruff` limpos (só `EXE002`). Nas 16.204
  vivas: 666.647 parágrafos, nenhuma vaga sem parágrafo, mediana 38 (Himalayas), 30 (Remote OK),
  29 (Remotive).
- Decisão: a `eda/` importa o `text.py` por um preâmbulo de `sys.path`, não instalando o
  `analyzer` como pacote. Testado: com `hatchling` editável funciona, mas no `docker run` o `uv run`
  reconstrói o projeto a cada partida do contêiner — risco desnecessário no Pi agora.
- Medição para a 2.3.2 (mentor): 450.067 parágrafos distintos; parágrafos em ≥2 vagas são 33,1%
  dos caracteres (≥5: 17,3%; ≥10: 11,2%; ≥50: 3,5%); 26,9% repetidos dentro de uma empresa só,
  6,3% entre empresas; `Originally posted on Himalayas` em todas as 16.049 da Himalayas; cortar
  ≥2 deixaria 962 vagas vazias, ≥10 deixaria 141, ≥50 nenhuma.

### 2026-10-21 — Subtarefa 2.3.2 (parágrafos repetidos)

- Do aluno: o `JOIN` com `sources`, `explode` com `ignore_index`, `jobs`/`companies` com
  `groupby().transform("nunique")` (e não `size`), A com `.loc[máscara, coluna]`, C com `&` e
  parênteses, D com `drop_duplicates` antes do `nlargest`, E com `groupby("job_id").any()` + `~`
  e o `<` estrito.
- Tropeços: `groupby("paragraph")` dentro da A (saía uma fração por texto em vez de um número) e
  `>` no lugar de `≥` (k = 2 dava 0.240 em vez de 0.331); a B com as fontes escritas à mão (o
  mentor refez com `where` + `groupby`, a pedido); um `.sum()` atribuído a uma coluna (o mesmo
  total em todas as linhas); `{k}` que sobrou do laço da A nos prints da C. A C só ficou clara com a
  vaga 3232 da Dell desmontada em C1/C2.
- Mentor (a pedido e pela preferência de prints/comentários): imports, `THRESHOLDS`, prints com
  `section()`, comentários com os achados, colunas da D escolhidas em vez de descartadas.
- ✅ **2.3.2 fechada.** Script roda do zero; `ruff` só `EXE002`. A: 0.331 · 0.173 · 0.112 · 0.035 ·
  0.025. B: Himalayas 0.333 · Remote OK 0.095 · Remotive 0.263. C: 0.269 numa empresa só · 0.063
  entre empresas. D: rodapé da Himalayas e títulos de seção. E: 962 · 274 · 141 · 0 · 0.
- Achados laterais: **D43** (parágrafos só de caracteres invisíveis) e **D44** (a mesma vaga
  publicada várias vezes com a cidade no título). Medição do mentor para a política do corte
  (a decisão está em "Decisões tomadas"): com o corte por vagas, quem fica quase vazio são
  famílias de vagas (Speechify por cidade, LanguageLine por idioma, Talkspace por estado, EWOR por
  área), e o que sobra é só o que muda entre elas (salário, idioma, licença).

### 2026-10-21 — Subtarefa 2.3.3 (corte de boilerplate) · Tarefa 2.3 fechada

- Do aluno: o `translate` no `_end_paragraph` (D43), `paragraphs_counter` com `set` por descrição,
  os testes da contagem (depois de trocar o caso 2, que testava a regra errada), o corte com list
  comprehension e `.get(p, 0)`, o "nunca vazia", os 5 casos do corte, a seção F até a fração por
  fonte.
- Tropeços: `from ../boilerplate import` (import não é caminho); o corte na direção invertida
  (percorria a contagem inteira e fazia `remove` na vaga: `ValueError` e um só `remove` por texto
  repetido) — ❌, refeito; `dict[str]int`; `jobs = jobs.groupby(...)` sem `.sum()`; a fração por
  fonte atribuída como coluna da `jobs` (índices diferentes → tudo `NaN`). Pediu a ideia da função
  "sem analogia": o fluxo HTML → parágrafos → corte → 3a/4b destravou.
- A pedido, o mentor fez: a tabela `REMOVE_INVISIBLE`, os 2 casos do D43, a regra 1 da contagem
  (`set(htmls)`) com `Counter.update`, o nome `cut_boilerplate` e a constante
  `BOILERPLATE_MIN_DESCRIPTIONS`, o caso real da Dell (`tests/testdata/dell_3232.html` e
  `dell_3234.html`), o total e as vagas inteiras da F, comentários e prints.
- Mutações (mentor): sem o `translate`, só os 2 casos do D43 falham; sem o `set` por descrição, o
  caso 2 da contagem falha; `<` → `<=`, sem o "nunca vazia" e `.get` → colchetes derrubam um caso
  cada; sem o `k=2`, o caso da Dell falha.
- ✅ **2.3.3 fechada · Tarefa 2.3 fechada.** `20 passed`, `mypy --strict` e `ruff` limpos (só
  `EXE002`). Corte nas vivas com k = 10: 0.102 do texto (Himalayas 0.102, Remote OK 0.015,
  Remotive 0.017); 2 vagas voltam inteiras. **D43 pago.**
- Medição para a 2.4 (mentor): Himalayas traz só países (250 nomes, nenhuma região); `Brazil` em
  324 vagas, lista vazia em 432. Com a regra do cartão da 2.4.1, só 338 das 16.204 vivas (2,1%)
  aceitam quem mora no Brasil, 501 são dúvida e 15.365 rejeitam. Idioma por palavras frequentes:
  ~173 em português, ~269 em espanhol.

---

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
| 2026-08-25 | Rotulagem contínua a partir do fim da Etapa 1 **Revista em 2026-09-29:** rotulagem concentrada na Fase 3b (`radar label`). |
| 2026-09-03 | `runs.fetched` = itens que a fonte devolveu; `runs.failed` = itens que não viraram linha em `raw_jobs` por erro | Duplicata (`ON CONFLICT DO NOTHING`) é **sucesso**, não falha — senão `failed` dispararia todo dia e viraria ruído. Idem para o elemento 0 da resposta do Remote OK, que é o aviso legal de ToS e nunca tem `id`. `jobs` fica fora da conta porque `normalizeRawJobs` varre a tabela inteira, não só a run. |
| 2026-09-03 | Falha de `INSERT` em `raw_jobs` rebaixada de fatal para erro por item | Um item com dado ruim não deve derrubar a coleta inteira. **Custo aceito:** se o banco cair no meio, todos os itens falham e o coletor ainda termina com `exit=0` e `error=NULL` — verificado: `fetched=101, failed=100, exit=0`. Falta um limiar de `failed/fetched`. Dono: Etapa 1. |
| 2026-09-03 | Abrir a linha de `runs` **antes** do `getSourceID` | Fonte ausente de `sources` é erro de configuração que acontece de madrugada. Abrindo antes, ela aparece em `runs` com `source_id` NULL em vez de sumir sem rastro. |
| 2026-09-09 | Um container Postgres por **pacote** de teste (`TestMain`), não por teste | Subir Postgres custa ~5s; um por teste inviabilizaria a Tarefa 0.3. **Custo aceito:** os testes passam a compartilhar estado, e a partir da 0.3.2 (que escreve no banco) vai ser preciso isolar — limpeza entre testes, e nada de `t.Parallel()` sem isso. |
| 2026-09-09 | Transação cobre **só** `normalizeRawJobs` + o laço de `insertJob`; `insertRawJobs` fica fora **Revista em 2026-10-03 (D29):** transação removida; erro de item é `Warn` + `continue`. |
| 2026-09-09 | Dentro da transação, tudo-ou-nada: `insertJob` retorna no primeiro erro, sem `continue` | Postgres aborta a transação inteira no primeiro statement com erro (`25P02`) — o `continue` virava 245 erros falsos e o `Commit` falhava do mesmo jeito. É aceitável porque `jobs` é derivação **pura** de `raw_jobs`: nada se perde, a run seguinte reconstrói. **Custo aceito:** atraso até a próxima coleta, e o risco de pílula envenenada (D19). || 2026-09-15 | Identidade mora no `.env`; topologia mora no arquivo que descreve o ambiente **Revista em 2026-10-03 (D29):** sem transação, não há mais tudo-ou-nada. |
| 2026-09-15 | `image:` mantido junto de `build:` no serviço `app` | Com as duas chaves, o compose constrói **e** aplica a tag dada, em vez de derivar `job-searcher-app` do nome do diretório. A Etapa 6 vai referenciar essa imagem num manifest que não sabe em que pasta o build ocorreu. |
| 2026-09-15 | `config.yaml` assado na imagem, não montado como volume | Mantém a imagem autocontida: `docker run` funciona sem `-v`. **Custo aceito e não discutido pelo aluno:** mudar uma linha do `config.yaml` exige rebuild da imagem, o que atrita com a promessa do §6 do SPEC ("ajusta o `config.yaml` e roda `docker compose up`"). Revisitar na 0.4.3 ou na Etapa 1, quando a lista de fontes começar a mudar de verdade. |
| 2026-10-08 | `config.yaml` assado na imagem **e** montado só-leitura por cima no compose (revisa a de 2026-09-15) | A imagem continua autocontida (`docker run` sem `-v` funciona) e no compose mudar o arquivo vale com `restart`, sem rebuild — paga o custo que a decisão de 2026-09-15 aceitou. Pegadinha registrada: bind mount de arquivo inexistente vira **diretório** no host. |
| 2026-10-08 | Espera inicial pelo banco no `main` (`waitForDB`), não `depends_on` | O `depends_on` só vale no `compose up`; no boot e no restart do Docker os containers voltam sem ordem. Esperar no próprio processo também cobre o k8s da Etapa 6, que não tem ordem de partida. Prazo de 2 min; esgotou → exit 1 e o `restart` tenta de novo. |
| 2026-10-16 | Duplicata = mesma empresa + título normalizados **e** (fonte diferente **ou** descrição idêntica) | Entre fontes a descrição nunca bate (HTML × texto), então a chave basta (2 grupos/semana, os dois a mesma vaga). Dentro da mesma fonte, descrição diferente pode ser outra vaga para quem procura remoto: a Bjak publica a mesma "Engineering Manager" híbrida e remota global, e só o bloco Location muda. Fundir esconderia a remota atrás da híbrida. **Custo aceito:** variações cosméticas da mesma vaga (106 grupos diferem em < 200 caracteres) continuam aparecendo repetidas. |
| 2026-10-17 | Normalização (`ToJob`) e dedupe exata ficam no Go; o Python não vira a camada de transformação do `raw_jobs` (proposta do aluno, revista e mantida "por enquanto") | O formato de cada fonte fica num lugar só: o Go já lê o payload para o `ExternalID`, e um `ToJob` em Python criaria dois parsers por fonte sem teste que pegue a divergência. Um processo basta para a vaga ficar pronta (o coletor provou 7 dias sozinho), e o reprocessamento sem recoleta já vem do `raw_jobs`. **Reabrir se:** a Fase 2 achar duplicatas que só a análise de texto pega (descrições que diferem só no boilerplate); essa camada seria do Python, por cima da do Go. |
| 2026-10-08 | `sleep` do client exportado (`client.Sleep`) em vez de copiado no `main` | Uma espera que respeita o `ctx`, num lugar só. **Custo aceito:** o pacote `client` (HTTP) passa a exportar um utilitário que não é de HTTP. Se aparecer um terceiro uso, mudar para um pacote próprio. |
| 2026-10-21 | Corte de boilerplate: um parágrafo é boilerplate quando aparece em **≥ 10 descrições distintas** (cópias idênticas contam uma vez); se o corte tirar todos os parágrafos de uma vaga, ela fica inteira | Medido nas vivas, com o D43 pago: k = 10 tira 10,2% do texto (Himalayas 10,2%, Remote OK 1,5%, Remotive 1,7%), 2 vagas caem no "nunca vazia". k = 5 tiraria 16,0%, mas 362 vagas ficariam com < 10% do texto; k = 50 só tira rodapé e títulos (3,2%). Contar descrições distintas protege as cópias: as 34 vagas da Speechify têm 7 descrições, e o texto da vaga fica. **Custo aceito:** famílias com ≥ 10 descrições quase iguais perdem o modelo comum (175 vagas com < 10% do texto) até o D44. Proposta do mentor; o limiar é uma constante, mudar é uma linha. |
| 2026-10-18 | ATS (Greenhouse, Lever, Ashby) e Arbeitnow em backlog, sem fase | Saíram da Fase 1 sem registro (o SPEC ainda os punha nela). A Fase 1 fechou com três boards; a EDA da Fase 2 diz se elas cobrem o que interessa antes de gastar tempo com fonte nova. |

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
| D10 | dados          | Payloads do Remote OK chegam com entidades HTML cruas (`&amp;`) e mojibake (`Donâ\u0080\u0099t`, `å\u008C\u0096`). Um `slug` vazio. `salary_min: 0` significa "não informado", não zero. | reagendado 2026-10-26 (3a.6) → **Fase 4b**: a 3a.2.1 provou que não afeta a extração de skills (Remote OK é 137/20.029 e mojibake/entidades estragam só não-ASCII, não os tokens ASCII das skills); só volta a doer quando o texto limpo for pro Jev (`salary_min: 0` já tratado na 3a.3.3) |
| ~~D11~~ | `main.go:54`   | `defer` roda durante panic e grava `error=NULL`: a run mente dizendo sucesso enquanto o processo morreu de SIGSEGV. Verificado em 2026-09-03. | ✅ pago 2026-10-05 (1.3.4) — `defer` com `recover` declarado **depois** do `defer` do `closeRun` (LIFO: roda antes) converte o panic em `panic: ...` no resultado nomeado; a fonte vira falha e as outras seguem. `TestPanic` com `Source` falsa |
| ~~D12~~ | `db/migrations/001_init.sql` | `runs` não tem `finished_at` — dá para saber quando começou e o que houve, não quanto durou. E não há limiar de `failed/fetched` que marque a run como fracasso. | ✅ pago 2026-10-05 — limiar na 1.3.3; `finished_at` (migration `006`, preenchido no `closeRun`) na 1.3.4 |
| D13 | `main_test.go:143` | `err := db.Close()` é sobrescrito pela linha seguinte: se o `Close` falhar e o `Terminate` funcionar, `teardown()` devolve `nil`. O `return err` final é sempre `nil`. | ✅ pago 2026-09-09 — `errors.Join` acumulando os dois |
| D14 | `main_test.go:90` | Loop `for rows.Next()` sem checar `rows.Err()` depois. | ✅ pago 2026-09-09 |
| D15 | `main.go:230` | `defer rows.Close()` vinha antes do `if err != nil`. Não quebra em pgx (`Query` sempre devolve `rows` não-nil), mas quebraria em `database/sql`. | ✅ pago 2026-09-09 |
| D16 | `main.go` | Nenhuma transação no projeto, apesar de "transação" estar nos conceitos da Tarefa 0.2. | ✅ pago 2026-09-09 — ver decisão da fronteira |
| ~~D17~~ | `Makefile:2,4,6` | Caminho absoluto `/mnt/ssd_m2/...` nos três alvos do goose: quebra em qualquer outra máquina, inclusive no CI da Etapa 7. Senha repetida três vezes (D4 com outro rosto). | ✅ pago 2026-10-18 (1.7.3, mentor) — `Makefile` lê `DATABASE_URL` do `.env`; nenhuma credencial no arquivo |
| ~~D18~~ | `main.go:246` | `logger.Info` por item: ~100 linhas INFO por execução escondendo as poucas que importam. `runs` já guarda a contagem. | ✅ pago 2026-10-07 — `Debug` por item + `source collected` por fonte em 2026-09-30 (0.5.1); o resto (`new raw_job inserted` em `insertRawJob`, reaberto no mesmo dia) virou `Debug` na limpeza geral |
| ~~D19~~ | `main.go` | Pílula envenenada: com tudo-ou-nada no laço de `insertJob`, um item permanentemente inválido faz **toda** run futura falhar, já que ele reaparece a cada coleta. Saída conhecida: savepoint por item (`tx.Begin()` sobre uma `Tx`). | ✅ pago 2026-10-03 junto com o D29 — sem transação no laço, erro de item é `Warn` + `failed++` + `continue`; um item inválido permanente não trava mais as runs seguintes (e, com o limiar da 1.3.3, 1 item em N não derruba a fonte). Constatado no fechamento da 1.3 |
| ~~D20~~ | `main.go:102` | `processSource` sem resultado nomeado: o `defer` lê o `err` **local**, então `runs.error` grava o erro cru sem o `fmt.Errorf` que envolve o retorno (verificado: `WARN` diz `failed to get source id: ...`, `runs.error` diz só `failed to query db: ...`). No dia que houver um `return fmt.Errorf("algo")` sem `err`, o `defer` grava `error=NULL` — D11 por outro caminho. | ✅ pago 2026-09-30 (0.5.1) — resultado nomeado; verificado com fonte `fake` (run 45) |
| ~~D21~~ | `main.go:99` | Se **todas** as fontes falharem, `run()` devolve `nil` e o processo sai com 0. Segundo caminho de falha silenciosa, irmão do D12 (limiar de `failed/fetched`). **Visto em 2026-09-30:** clone sem schema → `app` sai 0 → `analyzer` sobe e quebra. | ✅ pago 2026-10-05 (1.3.2) — `run` devolve `errors.Join` dos erros por fonte quando nenhuma teve sucesso (zero ligadas incluso); `TestClientTimeout` + `TestPartialFailure` |
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
| ~~D32~~ | `main.go` (`collectAndStoreData`) | O refactor da 1.1 trocou o `normalizeRawJobs` (varredura idempotente de `raw_jobs` → `jobs`) por "normalizar só o item recém-inserido + `if existed continue`". Dois efeitos: (1) `jobs` deixou de se auto-curar — `raw_job` que grava mas cujo `insertJob` falha vira **órfão** permanente (na run seguinte cai no `existed`); (2) matou o reprocessamento prometido no SPEC §4.1 ("rodar o normalizador de novo sobre o histórico"). | ✅ pago 2026-10-23 (3a.3.2) — `normalizeRawJobs` de volta: última versão por identidade (`ROW_NUMBER`), cursor no `id`, auto-cura o órfão, reprocessa o histórico; flag `-renormalize` one-shot; `TestNormalizeRawJobs` |
| D33 | `main.go` (`insertJob`) | O modelo `sources.Job` carrega `Date`, `Location` e `Tags`, mapeados com cuidado nas três fontes (incl. 3 formatos de data), mas `insertJob` grava só `title/company/url/description`. `Date` é calculado e descartado **mesmo havendo coluna `posted_at`**; `Location`/`Tags` não têm coluna ainda. Owners futuros concretos: Fase 2 (filtro de localização lê `Location`), Fase 3a ("idade da vaga" lê `posted_at`). | 🔶 parcial — `posted_at` + salário (`salary_min/max/currency/salary_period`) ✅ pagos 2026-10-23 (3a.3.1/3.3.3); `Location`/`Tags` ainda sem coluna → reagendado 2026-10-26 (3a.6) → **Fase 5** (digest é o consumidor de `location`; `tags` sem consumidor); 🟡 |
| ~~D34~~ | `internal/sources/himalayas.go:51` | Rótulo de erro copiado do Remotive + typo `localtion`. | ✅ pago 2026-10-03 (mentor, mecânico) — `remotive:` → `himalayas:`, `localtion` → `location`. |
| ~~D35~~ | `origin/main` (`d0296ea`) | Commit `docs: change docs` com marcador de conflito `=======` esquecido no `ROADMAP.md` e `PROGRESS.md` de volta ao estado pré-1.2. | ✅ pago 2026-10-05 — merge `8383222` de `feature/client` no `main` resolvido com `--theirs` nos dois docs; `main` idêntico ao branch, sem marcador. |
| D36 | `internal/sources/*.go` (`Fetch`) | Fonte que muda o formato da resposta (ex.: envelope `jobs` → `data`) faz o parser devolver **zero itens sem erro**. Pela regra da 1.3.3, zero itens não é falha (feed vazio pode ser legítimo, ex.: empresa de ATS sem vagas), então a quebra passa calada: `exit=0`, `error` NULL, `fetched=0`. | ✅ fechado 2026-10-08 (1.6.5) — `sourceLooksBroken` + `WARN source is broken` ao fim de cada ciclo; o parser continua devolvendo zero sem erro, mas não passa mais calado |
| ~~D37~~ | `internal/sources/remoteok.go:25` | `items[1:]` (pular o aviso legal do índice 0) entra em **panic** (`slice bounds out of range [1:0]`) se a API devolver `[]`. Hoje isso derruba o processo e grava a run com `error` NULL (D11). | ✅ pago 2026-10-18 (1.7.1) — `[]` vira `remoteok: empty feed, the legal notice is missing`; só o aviso = 0 itens sem erro; `TestRemoteOK` |
| D38 | `internal/client/client.go` (`HTTPGetBody`) | Com `Retries = 0` o laço não roda: nenhuma requisição é feita e a fonte falha com `maximum number of retries made: 0`. Hoje o valor é uma constante (3); na 1.4.3 vem do `config.yaml`, onde campo ausente = zero. Falha barulhenta (todas as fontes caem → `exit≠0`), mas com causa enganosa. Junto: o erro final não diz o último status (429 × 503), que antes da 1.4.1 ia para `runs.error`. **Irmão pior (1.4.2):** teto de páginas do Himalayas = 0 → nenhuma requisição, **zero itens e nenhum erro** — falha calada, classe do D36. Visto no `TestCursor` em 2026-10-07 (`Himalayas{}` sem o teto: 0 itens, 0 requisições). | 🔶 parcial 2026-10-07 (1.4.3) — zero/negativo em fonte ligada agora é erro na partida, no `Load`; o erro final diz o último status (`giving up after N attempts: last status S`, limpeza geral). Abertos: o teto continua no código (validar quando for para o config); `Config` montado sem `Load` (testes) ainda aceita zero |
| ~~D39~~ | `main.go` (`collector`) | Ciclo que falha **inteiro** (todas as fontes com erro) só tenta de novo no próximo tick, 6h depois. Na semana dos 7 dias, as 3 quedas de energia derrubaram o ciclo imediato por DNS (o roteador volta depois do Pi): 3 ciclos perdidos de ~33. O `waitForDB` não ajuda — o banco estava de pé; quem falta é a rede. Conserto provável: depois de um ciclo inteiro falho, tentar de novo em poucos minutos (com teto de tentativas) antes de voltar ao ritmo de 6h. | ✅ pago 2026-10-18 (1.7.2) — `runWithRetries`: 3 novas tentativas, 5 min entre elas, antes de voltar ao tick; `TestCollectorRetry` |
| D40 | `store.go` (marcação de duplicata, 1.5.3) | A marcação reavalia só a vaga **que está sendo gravada**. Se a **original** for editada e deixar de casar com as cópias (ex.: h1 muda a descrição; h2 e h3, da mesma fonte, continuam com a antiga), as cópias seguem apontando para ela e somem do digest sem ser mais duplicata. A cópia editada já se conserta sozinha: volta a `NULL` quando nenhuma candidata casa. | aberto — 🟡; alavanca manual pronta desde a 1.5.4: `UPDATE jobs SET company_key = NULL` faz o próximo ciclo reavaliar todas as vagas em ordem de `id`. Conserto automático não existe |
| D41 | `compose.yaml` (`analyzer`) | O `analyzer` espera só o `postgres` ficar saudável, não o `app`. No primeiro `up` de um clone limpo ele pode rodar antes das migrações (`UndefinedTable`, traceback cru — D26) ou antes da 1ª coleta (`0 of 0`). Hoje é contornado no README (`docker compose run --rm analyzer`). Não verificado na prática. | reagendado 2026-10-22 (2.5) → Fase 3a/5, quando o analyzer virar serviço real; 🟡 |
| D42 | `main.go:28` (`himalayasMaxPages = 20`) | O teto de 20 páginas (400 vagas) por ciclo está batendo: na maioria dos ciclos, 370–400 das 400 vagas lidas são inéditas, então a Himalayas publica mais em 6h do que o coletor lê, e parte das vagas nunca é vista. Medido em 2026-10-19 no dump local (runs 144–171; a 159 deu 0 por ser um restart 40 min depois da anterior). | aberto — 🟡, dono: Fase 2 (candidato à lista de defeitos da 2.2) — **defeito 7 do `docs/eda.md`** (47 runs: 37 de 44 cheias com ≥370 inéditas); decisão: consertar no Go, reagendado 2026-10-22 (2.5) → backlog (revisitar se a cobertura limitar a 3a) |
| D43 | `analyzer/text.py` (`_end_paragraph`) | `str.split()` não trata como espaço os caracteres invisíveis U+200B (zero-width space), U+FEFF (BOM), U+200C e U+200D. Parágrafos feitos só deles passam como não vazios: 554 linhas em 387 vagas vivas (U+200B 296, U+FEFF 187). Visto em 2026-10-21 no passo 4 da 2.3.2. | ✅ pago em 2026-10-21 (2.3.3): `REMOVE_INVISIBLE` no `text.py`, apagados antes de normalizar; 579 parágrafos a menos (666.068) — lista medida: U+200B, U+200C, U+200D, U+200E, U+2060, U+FEFF, U+00AD (todos da categoria Unicode `Cf`); 579 linhas só deles |
| D44 | dedupe (`store.go`, regra de 2026-10-16) | A mesma vaga publicada várias vezes com a cidade, o estado ou o idioma no título escapa da dedupe, porque o título muda. 609 vivas em 208 grupos têm a descrição byte a byte igual à de outra viva (401 cópias a mais); a maior é a Speechify, com `Team Lead, Android Core Product - <cidade>` 24 vezes. Fora as idênticas, há famílias em que só muda o salário ou o idioma (LanguageLine, Talkspace). Efeitos: o digest mostra a mesma vaga repetida, e o corte de boilerplate tira o modelo comum de famílias com ≥ 10 descrições (175 vivas ficam com < 10% do texto). Medido em 2026-10-21. | reagendado 2026-10-22 (2.5) → Fase 5 (consumidor é o digest); segue 🟡, gatilho "reabrir se" da decisão de 2026-10-17 |
| D45 | `analyzer/eda/06_language.py` (detector de idioma) | O detector por palavras de função confunde **português e espanhol** (e pega francês): ele mede "não-inglês", não PT-BR. Entre as vagas do balde `accept` da 2.4.1, 214 são "pt", mas desambiguando (`não/você/também` × `más/con/usted`) ~metade é espanhol (92 PT, 100 ES, 22 empate). Efeito nº 2 — **falso-negativo localização × idioma:** ~20 vagas em PT-BR puro caem em `reject` porque o `locationRestrictions` da Himalayas está errado (ex.: `[United States]` numa vaga claramente brasileira); nesses casos a língua é sinal de elegibilidade mais confiável que o país mal preenchido. Medido em 2026-10-22 (2.4.2). | aberto — 🟡, futuro (candidato à Fase 4b, quando o tratamento downstream diferir por língua): (a) detector PT-BR vs ES/PT-PT mais fino; (b) usar PT-BR como sinal fraco de elegibilidade quando a localização está vazia/suspeita |
| D46 | `store.go` (`insertJob`) + `analyzer/main.py` (`insert_job_skills`) | Skills defasam quando a descrição muda, por duas camadas: (1) `insertJob` reseta `scored_at` → `NULL` no `DO UPDATE` mas **não** `skills_extracted_at`, então vaga de texto alterado nunca volta pra fila de skills; (2) mesmo re-extraída, o `INSERT … ON CONFLICT (job_id, skill) DO UPDATE` só adiciona/atualiza — skill que sumiu da descrição fica como linha morta. Visto na 3a.2.2. | ✅ pago 2026-10-26 (3a.6) — texto muda → skills refeitas do zero: (1) `insertJob` reseta `skills_extracted_at` → `NULL` no `DO UPDATE` com o mesmo `CASE` do `scored_at` (vaga volta à fila de skills); (2) `insert_job_skills` apaga `job_skills` do `job_id` antes de reinserir (skill que sumiu some). `TestNilSkillsExtractedAt` (Go, testcontainer); prune conferido por observação no banco. `ON CONFLICT` do insert virou redundante (débito só do caminho feliz fechado) |
| D47 | `analyzer/skills.py` / `job_skills.confidence` | `confidence` é contagem crua de menções, não normalizada — infla com descrição longa e não é comparável entre vagas. O TF-IDF da 3a.5 não usa `confidence`. | reagendado 2026-10-26 (3a.6) → **Fase 4c** (é lá que `confidence` vira feature de modelo e a normalização passa a importar); 🟡 |
| D48 | `analyzer/skills.py` (`skills_extractor`) | O matcher não faz longest-match: `C` e `R` ficaram de fora do dicionário porque casariam em todo lugar, e `Shell` casa dentro de "Bash shell". Impacto baixo hoje (poucas skills de letra única; falso-positivo de `Shell` raro). | reagendado 2026-10-26 (3a.6) → **Fase 4c**/backlog, quando a precisão da extração de skills for medida contra rótulos; 🟡 |

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
- **2026-10-05** — `err != nil` prova que **algo** falhou; `errors.Is` prova que falhou **pelo
  motivo testado**. Na mutação "sem prazo" o `run` ainda devolve erro (de JSON) — um teste só com
  `err != nil` passaria com o timeout removido. (Respondido pelo aluno na pergunta da 1.3.2.)
- **2026-10-05** — dois prazos, vence o primeiro: o `Timeout` do client só enxerga o HTTP; o
  contexto da fonte corta tudo, banco incluso. Branch só protege o que foi commitado; `git restore`
  sem `--staged` descarta trabalho.
- **2026-10-05** — teste não cobre "todos os casos"; cobre as **bordas**. Com casos em 100%, 25% e
  vazio, trocar `>` por `≥` passa despercebido — só um caso em exatamente 50% distingue os dois.
  (Respondido pelo aluno na pergunta da 1.3.3.)
- **2026-10-05** — `defer` só protege o que roda **depois** de ele ser registrado. Panic dentro do
  `openRun` (antes dos dois `defer`) sobe sem `recover` e derruba o processo inteiro (exit 2); as
  fontes seguintes nem são tentadas. (Respondido pelo aluno na pergunta da 1.3.4.)
- **2026-10-06** — `select` é um ponto de espera com várias portas: bloqueia até um canal ficar pronto
  e executa só aquele caso. `time.Sleep` é esperar olhando só o relógio, com o celular desligado.
- **2026-10-06** — panic dentro de handler do `httptest` **não** derruba o teste: o `net/http`
  recupera e fecha a conexão, e o client vê erro de rede. Um caso que espera erro passa calado.
- **2026-10-06** — bom caso de teste faz o bug produzir um resultado **diferente** do correto, não
  só "outro erro": 404 seguido de 200 faz um client que reintenta 404 ter **sucesso**.
- **2026-10-06** — mesmo erro, caminho errado: ignorar o `ctx` na espera ainda termina em
  `DeadlineExceeded` (na requisição seguinte). Quando o erro não distingue, o tempo distingue.
- **2026-10-06** — `break` dentro de `switch`/`select` sai só dele, não do `for` que o envolve. O
  `break` da última tentativa no `client.go` funciona porque não há próxima volta — correto pela
  posição, não pela intenção. (Respondido pelo aluno na pergunta da 1.4.1.)
- **2026-10-07** — com "tudo ou nada" no `Fetch`, teto alto demais para o orçamento não coleta
  "parte": o prazo vence na página N, as N−1 páginas (variável local) são descartadas, `runs` grava
  `fetched=0` + `deadline exceeded`, e **toda** run seguinte repete — a fonte nunca mais coleta.
  Regra: teto × tempo por página cabe no orçamento, **com folga** (retry também gasta orçamento).
  (Pergunta da 1.4.2 respondida pelo mentor; o aluno acertou a conta de conferência: 1 req/2s em 60s
  → 30 páginas.)
- **2026-10-07** — retry também gasta orçamento: Himalayas com 120s, espera inicial 20s e 3
  tentativas → pior caso ≈ 61s por página (20s + 40s + 3 requisições); cabe 1 página, o teto 5 não.
  (Pergunta da 1.4.3 pulada pelo aluno; respondida pelo mentor.)
- **2026-10-07** — injetar o ciclo como função separa **quando** (o laço) de **o quê** (a coleta):
  o laço vira testável sem banco, com um ciclo falso que conta chamadas e cancela o `ctx`.
- **2026-10-07** — um teste de "continua depois do erro" só prova algo se o ciclo falso continua
  sendo chamado **depois** de falhar: cancelar na 1ª chamada faz o laço sair de qualquer jeito, e o
  teste não distingue "segue" de "desiste". Por isso cancelar vem antes de falhar.
- **2026-10-07** — o canal do `time.Ticker` guarda **no máximo 1** tick; os outros são descartados.
  Ciclo de 90s com intervalo de 1m: o tick dos 60s fica guardado, o ciclo seguinte começa colado
  (aos 90s), uma vez só — sem fila acumulando. (Pergunta da 1.6.1 pulada pelo aluno; respondida pelo
  mentor.)
- **2026-10-08** — derivar um contexto é **somar** o cancelamento do pai ao prazo do filho, não trocar
  um pelo outro: o primeiro que vencer encerra. Apagar o `WithTimeout` para "usar o ctx" tira o
  prazo — e no Pi ninguém aperta Ctrl+C.
- **2026-10-08** — `:=` dentro de um bloco cria variável nova se o nome não existe **naquele bloco**:
  o `err` do laço não é o `err` da função. Uma conferência que lê o `err` errado compila, passa no
  linter e nunca dispara — só um teste que derruba a mutação mostra que ela é morta.
- **2026-10-08** — o `Ticker` nasce **depois** do primeiro ciclo, então o 1º tick vem `interval`
  depois do **fim** dele; os seguintes vêm a cada `interval` a partir da criação, não importa quanto
  o ciclo dure. No 5b: 1ª coleta 14:53:35→40, tick às 14:54:40 (66s entre os logs), depois 59s.
  (Pergunta da 1.6.2 pulada pelo aluno; respondida pelo mentor.)
- **2026-10-08** — o `Ticker` tem período fixo, decidido na criação: dobrar a variável depois não
  muda nada. Backoff precisa de uma espera nova a cada volta (`Timer`/`client.Sleep` com a duração
  do momento).
- **2026-10-08** — função que devolve `(valor, err)`: não toque no valor antes de checar o `err`.
  O `openConn` devolve `nil` quando falha, e `Close` num `*pgx.Conn` nil é panic — `go vet` não pega.
- **2026-10-08** — Pi sem internet na partida (banco local de pé): a `waitForDB` passa, cada fonte
  falha depois dos retries (3 runs com `error`), o ciclo loga `no source succeeded` e o próximo só
  vem em 6h. A 1.6.4 não cobre isso, e não precisa: queda de internet não é corrida de partida. O
  que falta é **ver** que aconteceu — se durar K ciclos, o aviso da 1.6.5 mostra. (Pergunta da
  1.6.3 pulada pelo aluno; respondida pelo mentor.)
- **2026-10-08** — com `restart: unless-stopped`, o código de saída **não** muda a decisão do Docker:
  ele religa o container saia com 0 ou com 1, e só não religa se o container foi parado à mão
  (`docker stop`/`compose stop`) — nem depois de um restart do daemon. O código de saída importa
  para `on-failure` e para quem lê o `docker ps` (`Exited (1)` diz "morreu", `Exited (0)` diz
  "parou"). (Pergunta da 1.6.4 pulada pelo aluno; respondida pelo mentor.)
- **2026-10-16** — `docker compose down` remove os containers e, com eles, os **logs** (`json-file`
  mora em `/var/lib/docker/containers/<id>/`); os volumes ficam. `down -v` apaga também os volumes —
  o banco inteiro. Para parar sem perder log: `docker compose stop`.
- **2026-10-16** — container resolve nomes pelo DNS embutido do Docker (`127.0.0.11`), que repassa
  para o DNS do host. `server misbehaving` ali = o host sem DNS (roteador ainda bootando), não o site
  fora do ar. Três fontes falhando juntas com o mesmo erro de rede = problema de casa.
- **2026-10-17** — `[^a-z0-9]` apaga tudo o que não está na lista, **inclusive maiúsculas**: sem
  `ToLower` antes, "IBM" vira `""`. E um teste só pega o furo se tiver um caso onde o furo muda a
  resposta: Huzzle e Lemon.io têm as mesmas maiúsculas dos dois lados, então só a MWDN derruba a
  falta do `ToLower`.
- **2026-10-17** — struct literal pelo **nome** dos campos (`sources.Job{Company: …}`) deixa os
  outros no valor zero; pela **posição** exige todos, na ordem.
- **2026-10-18** — `defer` dentro de um `for` só roda quando a **função** termina, não a cada
  volta: num laço de lotes, o `rows` de cada volta fica aberto até o fim. Feche à mão (inclusive
  antes de cada `return`) ou extraia a volta para uma função própria.
- **2026-10-18** — `COUNT(col)` conta as linhas em que `col` não é `NULL`; `COUNT(DISTINCT col)`
  conta os valores diferentes. E uma consulta de aceite que dá 0 só prova algo se a mesma
  consulta, sem o filtro que conserta, der mais que 0.
- **2026-10-18** — teste que já passa **antes** do conserto não prova o conserto: o caso "feed
  vazio" com JSON inválido recebia um erro, só que de outro lugar. Ver o teste falhar pelo motivo
  certo é parte do teste.
- **2026-10-18** — um ticker sempre **espera e depois** dispara; para "tenta, e só se falhar
  espera", a espera vem depois da chamada (`Sleep` com o `ctx`). E teste com espera 0 não enxerga
  atraso nenhum: o `TestFirstCycle` só pegou o bug quando passou a usar uma espera de verdade.
- **2026-10-19** — `return 1` só vira código de saída quando alguém passa o retorno para o
  `sys.exit`; num script de exploração ninguém lê, e o erro passa calado. Função que não tem como
  cumprir o que promete levanta exceção. E `fetchone()` devolve a linha inteira, mesmo com uma
  coluna só.

---

## Retomada rápida

**Tarefas 1.6 e 1.5 fechadas** (Pi em `~/pi/job-searcher`, código `26cccd7` desde 2026-10-18,
coletando a cada 6h, com dedupe). O `jobs` local é a cópia do Pi de 2026-10-19
(dump em `db_dump/`; atualizar com dump novo + `dropdb`/`createdb`/`pg_restore --no-owner`); **o `app` local fica
parado** (`docker compose up -d postgres`, nunca o `up -d` completo). **Fase 1 fechada em
2026-10-18; Fase 2 aberta no mesmo dia.** Próximo: **2.4.1 — localização** (`eda/05_location.py`). Corte de
boilerplate pronto em `analyzer/boilerplate.py` (política em "Decisões tomadas", 2026-10-21); medição
em `eda/04_boilerplate.py`. Defeitos do dado em `docs/eda.md`; HTML → parágrafos em
`analyzer/text.py` (`html_to_paragraphs`, `uv run pytest` em `analyzer/`). Pi com a imagem de
`8498ca2` (D37 e D39 em produção) desde 2026-10-18. EDA em `analyzer/eda/`, rodada pelo `iron.nvim`
(REPL com `--all-groups` + `--env-file`) ou com `uv run --group eda --env-file ../.env ipython`
dentro de `analyzer/`. Dedupe: regra em `dedup.go`,
marcação em `store.go:updateDuplicate`, histórico em `store.go:backfillDedupe` (no `run`, antes das
fontes), aceite em `db/queries/live_duplicates.sql`. O mentor tem acesso de leitura ao Pi via `ssh pi` (`~/.ssh/config`). Antes do
`collector`, a `waitForDB` espera o banco com backoff por até 2 min; ao fim de cada ciclo,
`sourceLooksBroken` avisa fonte com 2 runs seguidas com erro ou 0 itens. O compose local já está
provado: `app` daemon com `unless-stopped`, `analyzer` roda uma vez, `config.yaml` montado `:ro`,
log `json-file` 10m×3, `stop` em < 1s mesmo no meio de uma coleta.
Código do `main` em `main.go` / `config.go` / `migrate.go` / `store.go`; o laço é o `collector`,
`-once` mantém a passada única; prazos derivam do `ctx` do sinal, o `closeRun` não. No
`config.yaml`, `collect_every` é o intervalo entre coletas, `timeout` é o **orçamento da fonte**,
timeout HTTP é `httpTimeout` (30s), `interval` é o rate limit. Abertos relevantes: D39 (ciclo falho
espera 6h), D38 (parcial), D37 (`items[1:]` do Remote OK com `[]`), D32, D33, D17, D25.

Estado verificado em 2026-10-18 (auditoria de fim da Fase 1): `gofmt`/`go vet`/`golangci-lint`
limpos, `go mod tidy` sem diferença, suíte verde com `-race` (`main` ~14s), `ruff`/`mypy` limpos
(o `EXE002` local é do disco NTFS — no git o `main.py` é `100644`). Só o `postgres` local de pé.
Pi: banco de 143 MB (`raw_jobs` 71 MB, 15.957 linhas; 15.279 vagas), disco em 41% de 29 GB.
Depois de mudar código, usar `docker compose up --build` (D25). A DSN do terminal usa a porta
`5433` (ver `.env.example`).
