INSERT INTO user_logins (user_id, login_time) SELECT u.id, NOW() FROM users u WHERE u.active;
