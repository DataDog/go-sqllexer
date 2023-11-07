UPDATE 
  products p
SET 
  price = CASE 
    WHEN p.stock < 10 THEN p.price * 1.10
    WHEN p.stock BETWEEN 10 AND 50 THEN p.price
    ELSE p.price * 0.90
  END,
  last_updated = NOW()
FROM (
  SELECT 
    product_id, 
    SUM(quantity) AS stock
  FROM 
    inventory
  GROUP BY 
    product_id
) AS sub
WHERE 
  sub.product_id = p.id;
