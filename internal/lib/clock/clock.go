package clock

import (
	"time"

	_ "time/tzdata"
)

const LocationName = "Europe/Moscow"

var Location = mustLoad()

func mustLoad() *time.Location {
	loc, err := time.LoadLocation(LocationName)
	if err != nil {
		panic(err)
	}
	return loc
}

func Now() time.Time {
	return time.Now().In(Location)
}
