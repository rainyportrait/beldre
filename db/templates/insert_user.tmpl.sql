INSERT INTO user (name, password)
VALUES (?, ?)
ON DUPLICATE KEY UPDATE id = id
