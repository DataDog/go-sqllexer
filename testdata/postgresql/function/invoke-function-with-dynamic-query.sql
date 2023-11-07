SELECT * FROM dynamic_query('SELECT * FROM users WHERE id = 1') AS t(id integer, name text, email text);
