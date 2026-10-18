SELECT
  COUNT(*) FILTER (
    WHERE
      n_sources > 1
  ) AS cross_source,
  COUNT(*) FILTER (
    WHERE
      n_sources = 1
      AND n_desc < n_rows
  ) AS same_source_repeated
FROM
  (
    SELECT
      COUNT(*) AS n_rows,
      COUNT(DISTINCT source_id) as n_sources,
      COUNT(DISTINCT description) as n_desc
    FROM
      jobs
    WHERE
      duplicate_of IS NULL
      AND company_key != ''
      AND title_key != ''
    GROUP BY
      (company_key, title_key)
    HAVING
      COUNT(*) > 1
  ) g;
