DELETE FROM sessions WHERE user_id = $1 AND expired = true;
