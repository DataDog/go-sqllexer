WITH deleted AS (
  DELETE FROM users WHERE last_login < NOW() - INTERVAL '2 years' RETURNING *
)
SELECT * FROM deleted;
