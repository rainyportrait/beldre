SELECT t.id, t.name
FROM post_tag pt
JOIN tag t ON t.id = pt.tag
WHERE pt.post = ?
ORDER BY t.name
