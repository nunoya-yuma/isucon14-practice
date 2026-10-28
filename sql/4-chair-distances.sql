-- Backfill chair_distances from the initial chair_locations; chairPostCoordinate keeps it up to date afterwards
INSERT INTO chair_distances (chair_id, total_distance, total_distance_updated_at, latitude, longitude)
SELECT chair_id, total_distance, created_at, latitude, longitude
FROM (SELECT chair_id,
             created_at,
             latitude,
             longitude,
             SUM(IFNULL(ABS(latitude - prev_latitude) + ABS(longitude - prev_longitude), 0))
                 OVER (PARTITION BY chair_id)                                AS total_distance,
             ROW_NUMBER() OVER (PARTITION BY chair_id ORDER BY created_at DESC) AS rn
      FROM (SELECT chair_id,
                   created_at,
                   latitude,
                   longitude,
                   LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)  AS prev_latitude,
                   LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at) AS prev_longitude
            FROM chair_locations) with_prev) ranked
WHERE rn = 1;
