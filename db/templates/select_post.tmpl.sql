SELECT
	p.id,
	p.source,
	p.uploader,
	u.name uploader_name,
	p.hash,
	pci.data crawl_info,
	p.created_at,
	p.updated_at
FROM post p
JOIN post_crawl_info pci ON pci.id = p.crawl_info
JOIN user u ON u.id = p.uploader
WHERE p.id = ?
