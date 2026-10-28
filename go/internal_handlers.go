package main

import (
	"database/sql"
	"math"
	"net/http"
)

type matchingChair struct {
	ID        string        `db:"id"`
	Latitude  sql.NullInt64 `db:"latitude"`
	Longitude sql.NullInt64 `db:"longitude"`
	Speed     int           `db:"speed"`
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rides := []Ride{}
	if err := db.SelectContext(ctx, &rides, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at`); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(rides) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// A chair is free once it has been notified of all 6 statuses of each of its rides,
	// so that it never sees a new ride before the COMPLETED of the previous one
	chairs := []matchingChair{}
	if err := db.SelectContext(ctx, &chairs, `SELECT chairs.id, chair_distances.latitude, chair_distances.longitude, chair_models.speed
FROM chairs
       INNER JOIN chair_models ON chair_models.name = chairs.model
       LEFT JOIN chair_distances ON chair_distances.chair_id = chairs.id
WHERE chairs.is_active = TRUE
  AND NOT EXISTS (SELECT 1
                  FROM rides
                         INNER JOIN ride_statuses ON ride_statuses.ride_id = rides.id
                  WHERE rides.chair_id = chairs.id
                  GROUP BY rides.id
                  HAVING COUNT(ride_statuses.chair_sent_at) < 6)`); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Oldest ride first so that no ride waits too long; each takes the chair that reaches its pickup soonest
	for _, ride := range rides {
		if len(chairs) == 0 {
			break
		}
		best := 0
		for i := range chairs {
			if pickupTime(chairs[i], ride) < pickupTime(chairs[best], ride) {
				best = i
			}
		}
		if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ? AND chair_id IS NULL", chairs[best].ID, ride.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		chairs = append(chairs[:best], chairs[best+1:]...)
	}

	w.WriteHeader(http.StatusNoContent)
}

// pickupTime estimates how long the chair takes to reach the ride's pickup point.
// Chairs that have never sent a location go last.
func pickupTime(c matchingChair, ride Ride) float64 {
	if !c.Latitude.Valid || !c.Longitude.Valid {
		return math.Inf(1)
	}
	d := calculateDistance(int(c.Latitude.Int64), int(c.Longitude.Int64), ride.PickupLatitude, ride.PickupLongitude)
	return float64(d) / float64(c.Speed)
}
