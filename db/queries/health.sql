SELECT
  date_trunc(
    'day',
    r.started_at AT TIME ZONE 'America/Sao_Paulo'
  )::DATE AS day,
  s.name AS source,
  COUNT(*) as runs,
  COUNT(r.error) AS errors
FROM
  runs r
  JOIN sources s ON s.id = r.source_id
WHERE
  r.started_at >=:'start'
GROUP BY
  source,
  day
ORDER BY
  day,
  source;

WITH
  d AS (
    SELECT
      source_id,
      started_at,
      started_at - lag(started_at) OVER (
        PARTITION BY
          source_id
        ORDER BY
          started_at
      ) AS gap
    FROM
      runs
    WHERE
      started_at >=:'start'
  )
SELECT
  s.name as source,
  max(d.gap) AS biggest_gap
FROM
  d
  JOIN sources s ON d.source_id = s.id
GROUP BY
  s.name
ORDER BY
  biggest_gap DESC NULLS LAST;
