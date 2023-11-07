DELETE FROM comments WHERE user_id IN (SELECT id FROM users WHERE status = 'banned');
