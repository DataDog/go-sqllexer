SELECT jsonb_set(data, '{info,address}', NULL) AS removed_address FROM users;
