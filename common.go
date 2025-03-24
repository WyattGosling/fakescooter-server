package main

import (
	"database/sql"
	"encoding/json"
)

// convention: structs with a Api suffix are used for JSON encoding/decoding
// and structs without are used for internal logic

type Optional[T any] struct {
	Defined bool
	Value   T
}

func (m Optional[T]) MarshalJSON() ([]byte, error) {
	if !m.Defined {
		return []byte("null"), nil
	}

	return json.Marshal(m.Value)
}

func (m *Optional[T]) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		m.Defined = false
		return nil
	}

	err := json.Unmarshal(data, &m.Value)
	if err != nil {
		return err
	}

	m.Defined = true
	return nil
}

type location struct {
	Latitude  float64 `json:"latitude"`  // North/South
	Longitude float64 `json:"longitude"` // East/West
}

type reservation struct {
	Active    sql.NullBool
	StartTime sql.NullInt64
}

type reservationApi struct {
	Active    bool  `json:"active"`
	StartTime int64 `json:"start_time,omitempty"`
}

type scooter struct {
	Id           string
	BatteryLevel int
	Location     location
}

type scooterIn struct {
	Id           Optional[string]   `json:"id,omitempty"`
	Reserved     Optional[bool]     `json:"reserved"`
	BatteryLevel Optional[int]      `json:"battery,omitempty"`
	Location     Optional[location] `json:"location,omitempty"`
}

type scooterOut struct {
	Id           string         `json:"id,omitempty"`
	Reservation  reservationApi `json:"reservation"`
	BatteryLevel int            `json:"battery,omitempty"`
	Location     location       `json:"location,omitempty"`
}

type user struct {
	Id   string
	Name string
}

type userApi struct {
	Id   Optional[string] `json:"id,omitempty"`
	Name Optional[string] `json:"name,omitempty"`
}
