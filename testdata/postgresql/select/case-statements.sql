SELECT name, CASE WHEN age < 18 THEN 'minor' ELSE 'adult' END FROM users;
