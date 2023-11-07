CREATE OR REPLACE FUNCTION get_user_count() RETURNS integer AS $func$
BEGIN
  RETURN (SELECT COUNT(*) FROM users);
END;
$func$ LANGUAGE plpgsql;
