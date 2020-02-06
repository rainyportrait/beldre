{{ if .Password }}
SELECT *
{{ else }}
SELECT
	id,
	name,
	level,
	created_at,
	updated_at,
{{ end }}
FROM user
WHERE name = ?
