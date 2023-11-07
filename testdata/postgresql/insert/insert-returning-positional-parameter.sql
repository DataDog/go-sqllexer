INSERT INTO orders (product_id, quantity, total) VALUES ($1, $2, $3) RETURNING id;
