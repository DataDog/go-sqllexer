SELECT jsonb_extract_path_text(data, 'user', 'name') AS user_name FROM user_profiles;
