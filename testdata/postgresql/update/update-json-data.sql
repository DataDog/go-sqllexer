UPDATE events SET data = jsonb_set(data, '{location}', '"New Location"') WHERE data->>'event_id' = '123';
