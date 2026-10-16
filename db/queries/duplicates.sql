WITH
  duplicate_groups AS (
    SELECT
      regexp_replace(lower(company), '[^a-z0-9]+', '', 'g') AS company_key,
      regexp_replace(lower(title), '[^a-z0-9]+', '', 'g') AS title_key,
      COUNT(*) AS jobs,
      COUNT(DISTINCT source_id) AS sources,
      COUNT(DISTINCT description) AS descriptions
    FROM
      jobs
    GROUP BY
      (company_key, title_key)
    HAVING
      COUNT(*) > 1
  )
SELECT
  CASE
    WHEN sources > 1 THEN 'cross_source'
    ELSE 'same_source'
  END AS kind,
  COUNT(*) AS groups,
  SUM(jobs) AS jobs,
  COUNT(*) FILTER (
    WHERE
      descriptions = 1
  ) AS same_description
FROM
  duplicate_groups
GROUP BY
  kind
ORDER BY
  kind;

WITH
  d AS (
    SELECT
      regexp_replace(lower(company), '[^a-z0-9]+', '', 'g') as company_norm,
      regexp_replace(lower(title), '[^a-z0-9]+', '', 'g') as title_norm
    FROM
      jobs
    GROUP BY
      (company_norm, title_norm)
    HAVING
      COUNT(DISTINCT source_id) > 1
  )
SELECT
  company,
  title,
  s.name
FROM
  jobs j
  JOIN sources s ON j.source_id = s.id
  JOIN d ON regexp_replace(lower(j.company), '[^a-z0-9]+', '', 'g') = d.company_norm
  AND regexp_replace(lower(j.title), '[^a-z0-9]+', '', 'g') = d.title_norm
ORDER BY
  j.company;
