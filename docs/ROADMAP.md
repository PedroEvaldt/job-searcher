# ROADMAP — job-radar

Só a etapa atual aparece em detalhe. As próximas ficam em uma linha.
Quando a Etapa 0 fechar, a Etapa 1 é expandida.

---

## Visão geral

| Etapa | Nome                        | Pronto quando                                                                                                         |
| ----- | --------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **0** | Uma fonte só, ponta a ponta | `docker compose up` coleta, o teste de integração passa e o Python enxerga o banco                                    |
| 1     | Coleta de verdade           | Mais fontes atrás da interface `Source`, dedup, rate limit, `runs` confiável — e o cron rodando sozinho em background |
| 2     | Olhar o dado                | EDA em Python sobre o que já foi coletado; você lista 5 defeitos concretos do seu próprio dado                        |
| 3     | Baseline honesta            | Extrator por dicionário, TF-IDF, `radar label`, `cv.md` — com um precision@10 medido                                  |
| 4     | ML de verdade               | sentence-transformers + pgvector, comparado contra a baseline nas mesmas vagas                                        |
| 5     | FastAPI e digest            | Analyzer expõe HTTP; primeiro digest chega no Discord sem você rodar nada à mão                                       |
| 6     | Kubernetes                  | Imagens multi-stage e multi-arch, minikube com CronJob/Deployment/ConfigMap/Secret/probes, Prometheus                 |
| 7     | Fechamento                  | CI no GitHub Actions, painel de tendências de stack, README que outra pessoa consegue seguir                          |

**Mudanças aplicadas ao plano original (Sessão Zero):**

- Prometheus saiu da Etapa 1 e foi para a 6, onde existe scraper de verdade. `runs` cobre observabilidade até lá.
- "Rodar 7 dias sozinho" deixa de ser portão bloqueante: o cron fica rodando em background **durante** a Etapa 2, que precisa do volume acumulado.
- Rotulagem vira contínua a partir do fim da Etapa 1 (alguns por semana), em vez de 200 de uma vez na Etapa 3.
- Currículo (`cv.md`) ganhou dono explícito: Etapa 3.
- Etapa 0 passou a atravessar Python também, com um `analyzer` mínimo.

---

## 📍 Etapa 0 — Uma fonte só, ponta a ponta

**Objetivo de aprendizado:** fazer o caminho mais fino possível atravessar Go, Postgres
e Python, provando que a integração funciona **antes** de qualquer feature. Ao fim
desta etapa, `docker compose up` numa máquina limpa coleta vagas reais e o Python
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
- [ ] **0.3.2** Alimentar o coletor com `testdata/remoteok.json` servido localmente e
      afirmar a idempotência de ponta a ponta.

### Tarefa 0.4 — Tudo dentro do compose, incluindo o Python

**Objetivo:** `docker compose up` numa máquina limpa faz o sistema inteiro funcionar,
e o Python prova que enxerga o mesmo banco.

**Critério de aceite:** clonar em diretório vazio, exportar as variáveis, `docker compose up`
→ o coletor roda e o `analyzer` imprime quantas vagas estão pendentes de score.

**Conceitos novos:** build multi-stage, `depends_on` com `condition`, healthcheck,
rede do compose, imagem distroless/alpine.

**Onde ler:** [Docker — Multi-stage builds](https://docs.docker.com/build/building/multi-stage/) ·
[Compose — depends_on](https://docs.docker.com/reference/compose-file/services/#depends_on)

- [ ] **0.4.1** Dockerfile multi-stage do `collector` + serviço no compose, esperando o
      Postgres ficar saudável.
- [ ] **0.4.2** `analyzer` Python mínimo no compose: conecta no Postgres e imprime a
      contagem de `jobs` com `scored_at is null`. Nada além disso.

---

## Etapas seguintes (resumo)

**Etapa 1 — Coleta de verdade.** Remotive e Himalayas entram; é olhando duas fontes lado a
lado que a interface `Source` nasce certa. Depois: dedup entre fontes, rate limit,
`config.yaml`, cron. Termina com o coletor rodando sozinho em background.

**Etapa 2 — Olhar o dado.** EDA em Python sobre o que se acumulou. Sai com 5 defeitos
concretos do seu próprio dado, escritos.

**Etapa 3 — Baseline honesta.** `cv.md`, extrator de skills por dicionário rodando nos dois
lados, TF-IDF, `radar label`, precision@10 medido.

**Etapa 4 — ML de verdade.** sentence-transformers, pgvector, comparação direta contra a
baseline usando `(job_id, model_version)`.

**Etapa 5 — FastAPI e digest.** `api` em Go, analyzer expondo HTTP, digest semanal no Discord.

**Etapa 6 — Kubernetes.** Multi-arch, minikube, manifests, probes, Prometheus.

**Etapa 7 — Fechamento.** CI, painel de tendências, README verificado por outra pessoa.
