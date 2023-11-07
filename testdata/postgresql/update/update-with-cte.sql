WITH updated AS (
  UPDATE users SET name = 'CTE Updated' WHERE id = 6 RETURNING *
)
SELECT * FROM updated;
