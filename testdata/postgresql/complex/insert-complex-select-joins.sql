INSERT INTO order_summaries (order_id, product_count, total_amount, average_product_price)
SELECT 
  o.id,
  COUNT(p.id),
  SUM(oi.amount),
  AVG(p.price)
FROM 
  orders o
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
GROUP BY 
  o.id
HAVING 
  SUM(oi.amount) > 1000;
