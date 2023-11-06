SELECT * FROM events WHERE payload @> '{"type": "user_event"}';
