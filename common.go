package main

type location struct {
	Latitude  float64 `json:"latitude"`  // North/South
	Longitude float64 `json:"longitude"` // East/West
}

type scooter struct {
	Id           string   `json:"id"`
	Reserved     bool     `json:"reserved"`
	BatteryLevel int      `json:"battery"`
	Location     location `json:"location"`
}

type scooterUpdate struct {
	Reserved     *bool     `json:"reserved"`
	BatteryLevel *int      `json:"battery"`
	Location     *location `json:"location"`
}

type user struct {
	Id          string  `json:"id"`
	Name        string  `json:"name"`
	Reservation *string `json:"reservation"`
}
