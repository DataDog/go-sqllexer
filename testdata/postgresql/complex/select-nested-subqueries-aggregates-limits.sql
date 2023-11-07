SELECT 
  user_id,
  order_id,
  order_total,
  user_total
FROM (
  SELECT 
    o.user_id,
    o.id AS order_id,
    o.total AS order_total,
    (SELECT SUM(total) FROM orders WHERE user_id = o.user_id) AS user_total,
    RANK() OVER (PARTITION BY o.user_id ORDER BY o.total DESC) AS rnk
  FROM 
    orders o
) sub
WHERE 
  sub.rnk = 1
AND user_total > (
  SELECT 
    AVG(total) * 2 
  FROM orders
);
