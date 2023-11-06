INSERT INTO user_logins (user_id, login_time) SELECT id, NOW() FROM users WHERE active;
