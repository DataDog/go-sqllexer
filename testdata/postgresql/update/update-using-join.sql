UPDATE orders SET total = total * 0.9 FROM users WHERE users.id = orders.user_id AND users.status = 'VIP';
