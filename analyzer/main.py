#!/usr/bin/env python3

import os
import sys

import psycopg


def main() -> int:
    database_url = os.getenv("DATABASE_URL")
    if database_url is None:
        print("DATABASE_URL nao setada", file=sys.stderr)
        return 1
    try:
        conn = psycopg.connect(database_url)
    except psycopg.OperationalError as e:
        detalhe = " ".join(str(e).split())
        print(f"Erro ao conectar no banco: {detalhe}", file=sys.stderr)
        return 1

    with conn, conn.cursor() as cur:
        cur.execute("""
                        SELECT COUNT(*) AS total,
                        COUNT(*) FILTER (WHERE scored_at IS NULL) AS sem_score
                        from jobs;
                        """)
        linha = cur.fetchone()
    assert linha is not None
    total, contagem = linha
    print(f"Tabela jobs tem {contagem} que nao foram feito score de {total} formulario")
    return 0


if __name__ == "__main__":
    sys.exit(main())
