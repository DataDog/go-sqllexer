SELECT user_data.name FROM (SELECT name FROM users WHERE active = true) AS user_data;
