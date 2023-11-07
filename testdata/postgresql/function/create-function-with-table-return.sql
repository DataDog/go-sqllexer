CREATE OR REPLACE FUNCTION get_users() RETURNS TABLE(user_id integer, user_name text) AS $func$
BEGIN
  RETURN QUERY SELECT id, name FROM users;
END;
$func$ LANGUAGE plpgsql;
