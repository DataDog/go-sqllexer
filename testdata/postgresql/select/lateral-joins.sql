SELECT u.name, json_agg(l) FROM users u, LATERAL (SELECT id, text FROM logs WHERE logs.user_id = u.id) AS l GROUP BY u.name;
