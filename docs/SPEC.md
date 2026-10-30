# SPEC — job-radar

> Escopo, stack e arquitetura. Decidido na Sessão Zero (2026-08-25).
> Mudanças aqui exigem `/replan`.

---

## 1. O que é

Ferramenta **self-hosted** que coleta vagas remotas internacionais de várias fontes,
ranqueia cada uma contra o currículo do usuário e manda um digest diário no Discord
(revisto 2026-10-30; era semanal).

Qualquer pessoa clona o repositório, escreve o próprio `cv.md`, ajusta o `config.yaml`,
exporta duas variáveis de ambiente e roda `docker compose up`.

**Não é** um job board, não é um agregador público, não tem multi-usuário,
não tem front-end web, nem servidor HTTP — o digest é o único jeito de ver o
resultado, de propósito (revisto 2026-10-30; FastAPI/`cmd/api` estavam previstos na
Sessão Zero sem necessidade concreta — ver §8).

---

## 2. Divisão de linguagens

O critério é: cada linguagem faz o que a outra faria mal.

| Go — plano de controle | Python — cérebro |
|---|---|
| I/O contra N fontes HTTP (hoje em sequência; concorrência entra quando uma fonte lenta justificar) | Extração de skills (NLP) |
| Rate limit e retry por fonte | Ranking, embeddings, modelo |
| Normalização e deduplicação | EDA e análise exploratória |
| CLI distribuível (binário único) | Ecossistema científico (pandas, sklearn, sentence-transformers) |
| Cron, daemons, agendamento | Iteração rápida sobre lógica de matching |

### Binários Go

| Binário | Papel |
|---|---|
| `collector` | Processo de longa duração com laço próprio: coleta na partida e a cada `collect_every` (desde a 1.6; antes era previsto cron). Busca as fontes, grava `raw_jobs`, normaliza para `jobs`, marca duplicatas, registra em `runs`. |
| `radar` | CLI do usuário: `init`, `label`, `digest`, `status`. |

### Serviço Python

| Serviço | Papel |
|---|---|
| `analyzer` | Laço próprio: acorda, procura trabalho pendente, extrai skills, calcula score, escreve em `scores`. |

---

## 3. Como os dois conversam

**Postgres é a fonte da verdade e o ponto de encontro.** Go e Python não compartilham
memória, socket, arquivo nem processo — só schema.

```
        ┌────────────────────────────────────────────────┐
        │                                                │
  ┌─────▼──────┐                                  ┌──────┴──────┐
  │  collector │  escreve raw_jobs, jobs, runs    │   analyzer  │
  │    (Go)    │─────────────┐      ┌─────────────│  (Python)   │
  └────────────┘             │      │             └─────────────┘
                             ▼      ▼               lê jobs WHERE
                       ┌──────────────────┐         scored_at IS NULL
  ┌────────────┐       │                  │         escreve job_skills,
  │   radar    │──────▶│    PostgreSQL    │◀────────scores
  │    (Go)    │       │                  │
  └────────────┘       └──────────────────┘
   lê jobs+scores, escreve labels
   digest → Discord (Fase 5)
```

**Trade-off aceito.** Banco compartilhado acopla os dois lados pelo schema: mudar uma
coluna quebra dois repositórios de código ao mesmo tempo, e não há contrato explícito
como haveria num gRPC. Em troca, ganhamos: zero infra de mensageria, cada lado pode
morrer e voltar sem perder trabalho, o estado é inspecionável com `psql`, e a fila de
trabalho sai de graça de um índice parcial. Para um único usuário e vazão de milhares
de vagas por dia, o custo do acoplamento é menor que o custo de uma fila.

**Consequência conhecida — concorrência.** O `analyzer` seleciona vagas pendentes.
Se duas instâncias subirem, ambas pegam as mesmas linhas e trabalham duas vezes.
O Postgres tem a resposta pronta para isso; ela será exigida na Fase 3.

---

## 4. Dados

Sete tabelas (`db/migrations/001_init.sql`). Duas decisões carregam peso:

### 4.1 `raw_jobs` separado de `jobs`

`raw_jobs` guarda o **payload cru** como veio da fonte, com `content_hash` e
`unique (source_id, external_id, content_hash)`. Reprocessar não exige recoletar —
quando o normalizador melhorar, ele roda de novo em cima do histórico.
Quando a fonte edita a vaga, o hash muda e nasce uma **nova linha** de histórico.

### 4.2 `jobs` = a vaga do mundo real (decisão da Sessão Zero)

Uma linha por vaga, **atualizada no lugar**. Não é fotografia.

Consequências obrigatórias:

- `jobs` precisa de **identidade estável própria** (`source_id` + `external_id`),
  independente de `raw_job_id`. O schema atual **não tem isso** — exige migration.
- Ao recoletar: se o conteúdo é idêntico, nada muda.
  Se mudou, a linha é atualizada e **`scored_at` volta a `NULL`**, jogando a vaga
  de volta na fila. Nota calculada sobre texto velho não vale para texto novo.
- `raw_job_id` aponta para a **versão mais recente** que originou o estado atual.

Alternativa descartada: `jobs` como snapshot por versão. Rejeitada porque empurraria a
complexidade para todo consumidor (`digest`, `label` teriam que escolher a versão
mais nova em toda consulta) e encheria `scores` de notas de versões mortas.

### 4.3 `scores` com chave `(job_id, model_version)`

Permite comparar versões do modelo sobre exatamente as mesmas vagas. É o que torna a
Fase 4 honesta: baseline e embeddings coexistem na tabela e são comparados de verdade,
não de memória.

### 4.4 `jev_answers` com chave `(job_id, question_set_version)`

Respostas do Jev (Fase 4b) guardadas por versão do question set. Mudar qualquer pergunta
exige subir o `question_set_version`, e ele entra no `model_version` de todo score que
dependa dessas respostas.

---

## 5. Fontes

Três tipos, dificuldade crescente:

| Tipo | Fontes | Dificuldade |
|---|---|---|
| **Boards com feed aberto** | Remote OK, Remotive, Himalayas (feitas, Fase 0–1); Arbeitnow (backlog) | JSON estruturado, sem auth. |
| **ATS por empresa** | Greenhouse, Lever, Ashby | Sem busca global — cobertura = tamanho da sua lista de empresas no `config.yaml`. Backlog. |
| **Comunidade** | Repositórios de vagas no GitHub (API de issues), Hacker News "Who is hiring" | Texto livre, sem estrutura. É o que **justifica NLP**. Hacker News por último. |

**Revisão de 2026-10-18 (fim da Fase 1):** a Fase 1 fechou com as três boards. ATS e Arbeitnow
saíram dela sem registro na época; ficam em backlog, sem fase, e voltam à mesa depois da EDA da
Fase 2 — se ela mostrar que as três fontes não cobrem o que interessa.

---

## 6. Currículo e configuração

### `cv.md`
Markdown, num diretório montado como volume. **Não** PDF, **não** GUI.

Razões: o `collector` roda em container sem display, então qualquer UI desktop seria
inútil onde o sistema realmente vive; parsing de PDF é custo sem aprendizado relevante
aqui; e markdown deixa o currículo no **mesmo formato textual da descrição de vaga**,
permitindo que **um único extrator de skills** rode nos dois lados. É isso que faz o
matching da Fase 3a fechar com um componente em vez de dois.

UI de configuração está fora de escopo (revisto 2026-10-30) — `config.yaml` editado
direto continua sendo o jeito.

### `config.yaml` (versionável, sem segredo)
- fontes ligadas/desligadas
- listas de empresas por ATS (Greenhouse/Lever/Ashby)
- intervalo do coletor
- caminho do currículo
- filtros duros: fuso horário aceitável, senioridade, palavras que descartam na hora
- nota mínima para entrar no digest
- horário do digest

### Variáveis de ambiente (nunca em arquivo versionado)
- `DATABASE_URL`
- `DISCORD_WEBHOOK_URL`
- `JEV_API_KEY` (opcional — sem ela, o projeto pontua com o cosseno)

---

## 7. Estrutura de diretórios (alvo)

O código **não** nasce assim. Ele chega aqui por pressão, quando a duplicação doer.
Isto é destino, não ponto de partida. Hoje (fim da Fase 1) o coletor é um `package main` na
raiz (`main.go`, `config.go`, `migrate.go`, `store.go`, `dedup.go`) mais `internal/client` e
`internal/sources`; os pacotes `internal/store` e `internal/config` e o `cmd/` nascem quando o
segundo binário (`radar`, Fase 3b) precisar deles (decisão de 2026-10-07).

```
job-radar/
├── cmd/
│   ├── collector/
│   └── radar/
├── internal/
│   ├── source/              # interface Source + uma impl por fonte
│   ├── store/               # acesso a Postgres
│   ├── normalize/
│   └── config/
├── analyzer/                # Python
│   ├── pyproject.toml
│   └── analyzer/
├── db/migrations/
├── testdata/
├── docs/
├── compose.yaml
└── config.yaml
```

---

## 8. Fora de escopo

Tão importante quanto o que está dentro.

- ❌ Multi-usuário, contas, autenticação
- ❌ Front-end SPA
- ❌ Aplicação automática a vagas
- ❌ Scraping de HTML de sites que não expõem API (fragilidade sem aprendizado)
- ❌ LinkedIn e Indeed (hostis a automação; risco jurídico e técnico)
- ❌ Cloud gerenciada / custo recorrente — tudo roda na máquina de quem clonou
- ❌ Treinar modelo de linguagem do zero (usar pré-treinados é o ponto)
- ❌ Notificação em tempo real — o produto é **digest em lote** (diário), deliberadamente
- ❌ Servidor HTTP / páginas web (API, dashboard, UI de config) — revisto 2026-10-30;
  FastAPI e `cmd/api` estavam previstos na Sessão Zero sem necessidade concreta que
  tivesse aparecido depois. O digest é o único jeito de ver o resultado, de propósito.
- ❌ Kubernetes / segundo caminho de deploy — revisto 2026-10-30 (era a Fase 6; ver §9).
  `docker compose up` no Raspberry Pi é o único alvo de deploy.

---

## 9. Kubernetes — descartado (decisão de 2026-10-30)

Previsto desde a Sessão Zero como uma Fase 6 só pelo portfólio e pelo aprendizado (nunca
uma necessidade funcional — `docker compose up` já bastava). Descartado porque: (a) não
há onde rodar um cluster de verdade além do próprio Raspberry Pi, que já roda o compose
sem problema; (b) o tempo que isso consumiria (imagens multi-arch, manifests, minikube)
compete com o que resta nas Fases 3-4 e o fechamento do projeto. O deploy do job-radar é
só `docker compose up` no Pi — não há dois caminhos paralelos.

---

## 10. Padrões inegociáveis

- Nenhum segredo hardcoded, desde o dia 1
- Toda execução do `collector` deixa rastro em `runs` — inclusive quando falha
- Migrations sempre com `up` **e** `down` (goose)
- Commits em inglês, imperativo, pequenos
- README atualizado ao fim de cada fase

### Avaliação

- O conjunto de teste congelado nunca entra em treino, ajuste de hiperparâmetro nem
  reescrita de pergunta
- O `radar label` nunca mostra score de modelo nenhum enquanto o aluno rotula
- Todo número reportado vem com o `model_version` que o gerou

---

## 11. Jev (a partir da Fase 4b)

LLM usado como **extrator de features**, não como juiz do match.

- Opcional no `config.yaml`: sem chave, o projeto pontua com o cosseno. Um teste prova isso.
- Provider trocado só pela base URL (TypeSafe, OpenRouter, OpenCode, Laya local). Nada
  acoplado a um provider específico.
- Perguntas em inglês, atômicas, com condições literais, **sempre sobre a vaga, nunca sobre
  o match**.
- Números, datas e salário são extraídos e comparados no código, nunca pelo Jev.
- O texto passa pelo corte de boilerplate (Fase 2) antes de ir para o Jev.
- Idioma da vaga detectado (`langdetect`, coluna `jobs.language`) e usado como critério direto da
  decisão final (ex.: francês não é enviado ao Jev) — não um lote PT-BR separado.
- Chamadas testáveis sem rede (resposta gravada em `testdata/`).
