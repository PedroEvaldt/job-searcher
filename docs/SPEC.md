# SPEC — job-radar

> Escopo, stack e arquitetura. Decidido na Sessão Zero (2026-08-25).
> Mudanças aqui exigem `/replan`.

---

## 1. O que é

Ferramenta **self-hosted** que coleta vagas remotas internacionais de várias fontes,
ranqueia cada uma contra o currículo do usuário e manda um digest semanal no Discord.

Qualquer pessoa clona o repositório, escreve o próprio `cv.md`, ajusta o `config.yaml`,
exporta duas variáveis de ambiente e roda `docker compose up`.

**Não é** um job board, não é um agregador público, não tem multi-usuário,
não tem front-end web (a não ser as páginas mínimas do FastAPI na Etapa 5).

---

## 2. Divisão de linguagens

O critério é: cada linguagem faz o que a outra faria mal.

| Go — plano de controle | Python — cérebro |
|---|---|
| I/O concorrente contra N fontes HTTP | Extração de skills (NLP) |
| Rate limit e retry por fonte | Ranking, embeddings, modelo |
| Normalização e deduplicação | EDA e análise exploratória |
| CLI distribuível (binário único) | Ecossistema científico (pandas, sklearn, sentence-transformers) |
| Cron, daemons, agendamento | Iteração rápida sobre lógica de matching |

### Binários Go

| Binário | Papel |
|---|---|
| `collector` | Roda no cron. Busca as fontes, grava `raw_jobs`, normaliza para `jobs`, registra em `runs`. |
| `radar` | CLI do usuário: `init`, `label`, `digest`, `status`. |
| `api` | Servidor HTTP. Nasce na Etapa 5. |

### Serviço Python

| Serviço | Papel |
|---|---|
| `analyzer` | Laço próprio: acorda, procura trabalho pendente, extrai skills, calcula score, escreve em `scores`. Vira serviço HTTP (FastAPI) na Etapa 5 **sem deixar de usar o banco**. |

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
   lê jobs+scores               ▲
   escreve labels               │
                          ┌─────┴──────┐
                          │    api     │  Etapa 5
                          │    (Go)    │
                          └────────────┘
```

**Trade-off aceito.** Banco compartilhado acopla os dois lados pelo schema: mudar uma
coluna quebra dois repositórios de código ao mesmo tempo, e não há contrato explícito
como haveria num gRPC. Em troca, ganhamos: zero infra de mensageria, cada lado pode
morrer e voltar sem perder trabalho, o estado é inspecionável com `psql`, e a fila de
trabalho sai de graça de um índice parcial. Para um único usuário e vazão de milhares
de vagas por dia, o custo do acoplamento é menor que o custo de uma fila.

**Consequência conhecida — concorrência.** O `analyzer` seleciona vagas pendentes.
Se duas instâncias subirem, ambas pegam as mesmas linhas e trabalham duas vezes.
O Postgres tem a resposta pronta para isso; ela será exigida na Etapa 3.

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
complexidade para todo consumidor (`digest`, `label`, `api` teriam que escolher a versão
mais nova em toda consulta) e encheria `scores` de notas de versões mortas.

### 4.3 `scores` com chave `(job_id, model_version)`

Permite comparar versões do modelo sobre exatamente as mesmas vagas. É o que torna a
Etapa 4 honesta: baseline e embeddings coexistem na tabela e são comparados de verdade,
não de memória.

---

## 5. Fontes

Três tipos, dificuldade crescente:

| Tipo | Fontes | Dificuldade |
|---|---|---|
| **Boards com feed aberto** | Remote OK, Remotive, Himalayas, Arbeitnow | JSON estruturado, sem auth. Etapa 0–1. |
| **ATS por empresa** | Greenhouse, Lever, Ashby | Sem busca global — cobertura = tamanho da sua lista de empresas no `config.yaml`. Etapa 1. |
| **Comunidade** | Repositórios de vagas no GitHub (API de issues), Hacker News "Who is hiring" | Texto livre, sem estrutura. É o que **justifica NLP**. Hacker News por último. |

---

## 6. Currículo e configuração

### `cv.md`
Markdown, num diretório montado como volume. **Não** PDF, **não** GUI.

Razões: o `collector` roda em container sem display, então qualquer UI desktop seria
inútil onde o sistema realmente vive; parsing de PDF é custo sem aprendizado relevante
aqui; e markdown deixa o currículo no **mesmo formato textual da descrição de vaga**,
permitindo que **um único extrator de skills** rode nos dois lados. É isso que faz o
matching da Etapa 3 fechar com um componente em vez de dois.

Se um dia houver UI de configuração, ela nasce como página do FastAPI na Etapa 5.

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

---

## 7. Estrutura de diretórios (alvo)

O código **não** nasce assim. Ele chega aqui por pressão, quando a duplicação doer.
Isto é destino, não ponto de partida.

```
job-radar/
├── cmd/
│   ├── collector/
│   ├── radar/
│   └── api/                 # Etapa 5
├── internal/
│   ├── source/              # interface Source + uma impl por fonte
│   ├── store/               # acesso a Postgres
│   ├── normalize/
│   └── config/
├── analyzer/                # Python
│   ├── pyproject.toml
│   └── analyzer/
├── db/migrations/
├── deploy/k8s/              # Etapa 6
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
- ❌ Notificação em tempo real — o produto é **digest semanal**, deliberadamente

---

## 9. Tensão registrada: compose vs Kubernetes

O produto promete `docker compose up`. A Etapa 6 entrega Kubernetes.

São **dois caminhos de deploy paralelos**, não uma migração. O k8s existe pelo portfólio
e pelo aprendizado; o compose é o contrato com quem clona o repositório. Regra: nenhuma
mudança da Etapa 6 pode quebrar o `docker compose up`. O README continua ensinando compose.

---

## 10. Padrões inegociáveis

- Nenhum segredo hardcoded, desde o dia 1
- Toda execução do `collector` deixa rastro em `runs` — inclusive quando falha
- Migrations sempre com `up` **e** `down` (goose)
- Commits em inglês, imperativo, pequenos
- README atualizado ao fim de cada etapa
