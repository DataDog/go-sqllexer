SELECT * FROM users WHERE last_login < NOW() - INTERVAL '1 year' FOR UPDATE OF users;
