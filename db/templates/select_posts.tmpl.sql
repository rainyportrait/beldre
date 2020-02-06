SELECT
	p.id,
	p.source,
	p.hash,
	p.created_at,
	p.updated_at
FROM
	post p,
	post_tag pt,
	tag t
WHERE
	pt.tag = t.id
	AND p.id = pt.post
	{{ if .Tags }}
	AND t.name IN (?)
	{{ end }}
	{{ if.Exclude }}AND p.id NOT IN (
		SELECT p.id
		FROM
			post p,
			post_tag pt,
			tag t
		WHERE
			pt.tag = t.id
			AND p.id = pt.post
			AND t.name IN (?)
	)
	{{ end }}
GROUP BY p.id
{{ if .Tags }}
HAVING COUNT(p.id) = ?
{{ end }}
ORDER BY {{ .Order }}
LIMIT ? OFFSET ?
