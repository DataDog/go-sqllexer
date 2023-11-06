WITH recursive_subordinates AS (
    SELECT id, manager_id FROM employees WHERE id = 1
    UNION ALL
    SELECT e.id, e.manager_id FROM employees e INNER JOIN recursive_subordinates rs ON rs.id = e.manager_id
)
SELECT * FROM recursive_subordinates;
