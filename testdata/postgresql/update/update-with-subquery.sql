UPDATE products SET price = (SELECT MAX(price) FROM products) * 0.9 WHERE name = 'Old Product';
