SELECT
	t.id,
	t.name,
	COUNT(1) count
FROM post_tag pt
JOIN tag t ON pt.tag = t.id
WHERE t.id IN
	(
		SELECT t.id
		FROM post_tag pt
		JOIN tag t ON pt.tag = t.id
		WHERE pt.post IN (?)
	)
GROUP BY t.name
ORDER BY count DESC
LIMIT 25
