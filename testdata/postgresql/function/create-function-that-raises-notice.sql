CREATE OR REPLACE FUNCTION log_activity(activity text) RETURNS void AS $func$
BEGIN
  RAISE NOTICE 'Activity: %', activity;
END;
$func$ LANGUAGE plpgsql;
