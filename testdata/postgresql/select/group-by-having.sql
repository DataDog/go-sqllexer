SELECT status, COUNT(*) FROM orders GROUP BY status HAVING COUNT(*) > 1;
