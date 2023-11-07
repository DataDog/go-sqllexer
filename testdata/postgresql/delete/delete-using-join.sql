DELETE FROM user_logins USING users WHERE user_logins.user_id = users.id AND users.status = 'inactive';
