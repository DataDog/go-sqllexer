SELECT 
  u.id,
  u.name,
  (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id) AS order_count,
  (SELECT SUM(amount) FROM payments p WHERE p.user_id = u.id) AS total_payments,
  (SELECT AVG(rating) FROM reviews r WHERE r.user_id = u.id) AS average_rating
FROM 
  users u
WHERE 
  EXISTS (
    SELECT 1 FROM logins l WHERE l.user_id = u.id AND l.time > NOW() - INTERVAL '1 month'
  )
AND u.status = 'active'
ORDER BY 
  total_payments DESC, average_rating DESC, order_count DESC
LIMIT 10;
