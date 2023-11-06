SELECT jsonb_set(data, '{user,name}', '"John Doe"') AS updated_data FROM user_profiles;
