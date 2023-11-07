CREATE OR REPLACE FUNCTION get_user_email(user_id integer) RETURNS text AS $func$
BEGIN
  RETURN (SELECT email FROM users WHERE id = user_id);
END;
$func$ LANGUAGE plpgsql;
