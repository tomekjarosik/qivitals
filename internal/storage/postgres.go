package storage

/*
SELECT * FROM sensors
WHERE namespace = 'db'
  AND labels @> '{"env": "prod"}'      -- labels map
  AND labels ? 'critical'               -- has_label_keys
  AND status IN ('DEGRADED', 'DEAD')    -- statuses
  AND (name ILIKE '%backup%' OR description ILIKE '%backup%') -- search
ORDER BY last_updated_timestamp DESC
LIMIT 50;
```
*/
