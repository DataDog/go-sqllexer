DELETE FROM 
  users u
USING 
  orders o,
  order_items oi,
  products p
WHERE 
  u.id = o.user_id
AND o.id = oi.order_id
AND oi.product_id = p.id
AND p.category = 'obsolete'
AND o.order_date < NOW() - INTERVAL '5 years';
