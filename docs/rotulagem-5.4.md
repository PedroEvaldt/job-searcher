# Rotulagem da 5.4 — passo a passo

Guia de execução das duas rodadas de rótulos da Tarefa 5.4:

- **Rodada 1 (5.4.1):** 200 vagas sorteadas → 75 viram teste novo, 125 viram treino.
- **Rodada 2 (5.4.4):** 150 vagas de treino, as do topo da regra `skills-gate-v1`.

Tudo roda no **desktop**, que é onde está o banco. Siga na ordem. Cada passo tem um
✔️ que diz como saber que ele deu certo; não avance sem ele.

---

## 0. Antes de tudo (uma vez só)

### 0.1 No notebook: subir o código

O código dos scripts precisa chegar ao desktop. No notebook:

```bash
git status                     # nada de scripts/match pendente?
git push
```

✔️ `git status` diz `Your branch is up to date with 'origin/main'` e não lista
`match.py`, `score_skills_gate.py` nem `build_label_queue_round2.py`.

### 0.2 No desktop: atualizar e preparar

```bash
cd ~/caminho/do/job-searcher   # raiz do repositório
git pull
docker compose up -d postgres
```

✔️ `docker compose ps postgres` mostra `healthy`.

### 0.3 Conferir o `cv.md`

O `cv.md` **não vai pelo git** (está no `.gitignore`). A regra da rodada 2 lê as skills dele.
Ele precisa existir na raiz do repositório do desktop, e ser o mesmo CV do notebook.

```bash
ls cv.md
cd analyzer && uv run --group dev pytest tests/test_match.py -q && cd ..
```

✔️ `ls` acha o arquivo e o pytest passa (um dos testes lê o `cv.md` de verdade e exige Go e
Python nele).

### 0.4 Compilar o `radar` e carregar o `.env` no terminal

O `radar` lê `DATABASE_URL` do ambiente; ele **não** lê o `.env` sozinho. Os scripts Python
leem com `--env-file`, então só o radar precisa disso.

```bash
go build -o cmd/radar/radar ./cmd/radar
set -a && . ./.env && set +a   # vale só para ESTE terminal; repita se abrir outro
./cmd/radar/radar status
```

✔️ `status` imprime `LABELED <feitas>/<restantes>`. Anote o número de feitas (são os rótulos
antigos).

---

## Rodada 1 — 200 aleatórias

### 1.1 Sortear (uma vez só, nunca de novo)

```bash
cd analyzer
uv run --env-file ../.env python -m scripts.sample_label_round
cd ..
```

✔️ Saída parecida com
`drew 200 of 19xxx unlabelled live jobs: 75 test + 125 train; test-set-v2 has 150 jobs`,
e existem `analyzer/data/frozen.json` e `analyzer/data/label_queue_round1.json`.

Se ele recusar dizendo que os arquivos já existem, **pare**: o sorteio já foi feito. Não apague
nada sem antes falar com o mentor.

### 1.2 Commitar o sorteio ANTES de rotular

É isso que torna o teste honesto: o git prova que as 150 vagas de teste foram escolhidas antes
de você ver qualquer uma delas.

```bash
git add analyzer/data/frozen.json analyzer/data/label_queue_round1.json
git commit -m "data: freeze test-set-v2 and the round 1 label queue"
git push
```

✔️ `git log -1 --stat` mostra os dois arquivos.

### 1.3 Rotular

Na raiz do repositório, no terminal onde você rodou o `set -a` (0.4):

```bash
./cmd/radar/radar label analyzer/data/label_queue_round1.json
```

- Cada vaga aparece com `[N of 200 left in the queue]` no topo.
- **Um `label` para depois de 100 vagas.** As 200 levam no mínimo duas execuções. Divida em
  **pelo menos 2 sessões, em dias diferentes se der**: cansado, você rotula pior.
- **Ctrl+D** para no meio de qualquer vaga. A vaga da tela **não** é gravada; ela volta na
  próxima execução. Rode o mesmo comando para continuar de onde parou.
- Cada resposta é gravada na hora.

**Como responder:**

| Pergunta | O que conta |
|---|---|
| `Would apply (y/n)` | **O único que importa.** "Eu mandaria currículo pra essa vaga, hoje?" |
| `Hires Latam` | Opcional, anotação. Enter vazio = não sei. |
| `Seniority` | Opcional, anotação. Enter vazio = não sei. |
| `Note` | Opcional. Use quando ficar em dúvida: ajuda a auditar depois. |

Mantenha o **mesmo critério dos rótulos antigos**. Se uma vaga parecer igual a outra já
vista (repost), responda igual. Teste e treino estão misturados de propósito; você não sabe
qual é qual, e é assim que deve ser.

**Errou uma resposta?** Apague o **último** rótulo gravado, e a vaga volta para a fila:

```bash
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "DELETE FROM labels WHERE job_id = (SELECT job_id FROM labels ORDER BY labeled_at DESC LIMIT 1) RETURNING job_id;"'
```

Saia do `label` com Ctrl+D antes de rodar isso e entre de novo depois.

**Backup ao fim de cada sessão** (os rótulos só existem neste banco; o Pi não tem):

```bash
mkdir -p db_dump
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t labels --data-only' > db_dump/labels-$(date +%F).sql
```

✔️ No fim, `label` imprime `nothing left to label` e `radar status` mostra ~200 feitas a mais
que o número anotado em 0.4.

### 1.4 Medir a linha de base

```bash
cd analyzer
uv run --env-file ../.env python -m scripts.evaluate
cd ..
```

✔️ Saída sem `warning`:

```
test-set-v2: n=150 pos=… (random AP=…)
jev-manual-qs-v1  AP=…  ROC AUC=…
```

- `warning: N of 150 test jobs not labelled yet` → a rodada 1 não acabou; volte a 1.3.
- `no jev-manual-qs-v1 score for jobs: [...]` → alguma vaga de teste nova não tem a nota da
  jev-manual. Rode `uv run --env-file ../.env python -m scripts.score_jev` (dá nota a todo o
  teste a partir das respostas do Jev) e repita.

📨 **Mande essa saída ao mentor.** É a linha de base que a regra das skills precisa bater.

---

## Rodada 2 — 150 do topo da regra

Só comece com a rodada 1 **inteira** (o script da fila 2 lê a fila 1 para não repetir vagas).

### 2.1 Extrair skills das vagas que ainda não têm

```bash
cd analyzer
uv run --env-file ../.env python -m scripts.main
```

✔️ Roda até o fim sem erro. (Na primeira vez pode demorar alguns minutos, se muitas vagas novas
entraram desde a última extração.)

### 2.2 Dar a nota da regra a todas as vagas vivas

```bash
uv run --env-file ../.env python -m scripts.score_skills_gate
```

✔️ Saída parecida com
`skills-gate-v1: scored N live jobs, M with score > 0; skipped K without Jev answers (cv: 12 skills)`.

- `cv: 12 skills`: se vier outro número, o `cv.md` do desktop é diferente do que decidimos (0.3).
- `warning: … have no extracted skills yet` → o 2.1 não terminou; rode de novo.
- `K` grande → muitas vagas sem resposta do Jev ainda. Elas ficam fora da rodada 2; tudo bem,
  mas anote o número.

### 2.3 Comparar a regra com a linha de base

```bash
uv run --env-file ../.env python -m scripts.evaluate jev-manual-qs-v1 skills-gate-v1
```

✔️ Duas linhas, `jev-manual-qs-v1` e `skills-gate-v1`, ambas com AP. A da jev-manual tem que ser
**a mesma** de 1.4 (o teste não mudou).

📨 **Mande essa saída ao mentor.** É o primeiro número que diz se as skills ajudam.

### 2.4 Montar a fila 2 (uma vez só, nunca de novo)

```bash
uv run --env-file ../.env python -m scripts.build_label_queue_round2
cd ..
```

✔️ Saída parecida com `round 2: 150 jobs from the top of skills-gate-v1 (score …–…, 150 in the gate)`
e existe `analyzer/data/label_queue_round2.json`.

- `warning: only X of 150 have score > 0` → não é erro. Menos de 150 vagas passaram no filtro, e o
  resto da fila são vagas de nota 0. Mande o número ao mentor.
- `only X candidates — did score_skills_gate run?` → volte a 2.2.

### 2.5 Commitar a fila ANTES de rotular

```bash
git add analyzer/data/label_queue_round2.json
git commit -m "data: freeze the round 2 label queue"
git push
```

### 2.6 Rotular

Mesmas regras de 1.3 (terminal com `set -a`, Ctrl+D para parar, backup no fim da sessão):

```bash
./cmd/radar/radar label analyzer/data/label_queue_round2.json
```

A ordem está embaralhada: a 1ª vaga da fila não é a de nota mais alta. Espere **mais "y" que na
rodada 1**, porque estas vagas vêm do topo da regra. Mesmo assim, responda pelo seu critério,
não pela expectativa. Os "n" desta rodada são justamente os erros da regra que o modelo da 5.4.5
precisa aprender.

✔️ `nothing left to label`, e `radar status` mostra ~150 feitas a mais que no fim da rodada 1.

📨 **Avise o mentor.** Próximo: 5.4.5, treinar o `sklearn-combined-v2` com os ~450 rótulos de treino.

---

## Resumo de uma tela

```bash
# rodada 1
uv run --env-file ../.env python -m scripts.sample_label_round        # analyzer/
git add analyzer/data/frozen.json analyzer/data/label_queue_round1.json && git commit -m "data: freeze test-set-v2 and the round 1 label queue"
./cmd/radar/radar label analyzer/data/label_queue_round1.json         # raiz, 2+ vezes
uv run --env-file ../.env python -m scripts.evaluate                  # analyzer/ → mandar ao mentor

# rodada 2
uv run --env-file ../.env python -m scripts.main                      # analyzer/
uv run --env-file ../.env python -m scripts.score_skills_gate
uv run --env-file ../.env python -m scripts.evaluate jev-manual-qs-v1 skills-gate-v1   # → mandar ao mentor
uv run --env-file ../.env python -m scripts.build_label_queue_round2
git add analyzer/data/label_queue_round2.json && git commit -m "data: freeze the round 2 label queue"
./cmd/radar/radar label analyzer/data/label_queue_round2.json         # raiz, 2+ vezes
```
