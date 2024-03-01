CREATE TABLE IF NOT EXISTS deployment (uuid TEXT PRIMARY KEY, generation_uuid TEXT, start_at INT, end_at INT, error_msg TEXT, restart_comin INTEGER, status TEXT, operation TEXT);
