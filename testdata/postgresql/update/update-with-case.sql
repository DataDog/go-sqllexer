UPDATE users SET status = CASE WHEN last_login < NOW() - INTERVAL '1 year' THEN 'inactive' ELSE status END;
