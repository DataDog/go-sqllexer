UPDATE users SET last_login = NOW() WHERE id = 3 RETURNING last_login;
