package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

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

var defaultScooters = []scooter{
	{Id: "abc123", Reserved: false, BatteryLevel: 99, Location: location{Latitude: 49.26227, Longitude: -123.14242}},
	{Id: "def456", Reserved: false, BatteryLevel: 88, Location: location{Latitude: 49.26636, Longitude: -123.14226}},
	{Id: "ghi789", Reserved: true, BatteryLevel: 77, Location: location{Latitude: 49.26532, Longitude: -123.13659}},
	{Id: "jkl012", Reserved: false, BatteryLevel: 9, Location: location{Latitude: 49.26443, Longitude: -123.13469}},
}

var defaultUsers = map[string]user{
	"a1": {Id: "a1", Name: "pay2go", Reservation: nil},
	"b2": {Id: "b2", Name: "basic", Reservation: nil},
	"c3": {Id: "c3", Name: "premium", Reservation: nil},
}

func doAuthStuff(header *http.Header) (user, error) {
	auth := header.Get("Authorization")
	if auth == "" {
		return user{}, errors.New("no authorization provided")
	}

	// assuming Basic auth
	bytes, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return user{}, nil
	}

	authStr := string(bytes)
	splits := strings.Split(authStr, ":")
	username := splits[0]
	for _, candidate := range defaultUsers {
		if candidate.Name == username {
			return candidate, nil
		}
	}

	return user{}, errors.New("unknown user")
}

func checkLocation(loc location) error {
	badLat := false
	if loc.Latitude > 180 || loc.Latitude < -180 {
		badLat = true
	}

	badLon := false
	if loc.Longitude > 180 || loc.Latitude < -180 {
		badLon = true
	}

	if badLat && badLon {
		return errors.New("values for location.latitude and location.longitude must be in range [-180, 180]")
	} else if badLat {
		return errors.New("value for location.latitude must be in range [-180, 180]")
	} else if badLon {
		return errors.New("value for location.longitude must be in range [-180, 180]")
	} else {
		return nil
	}
}

func getScootersHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(defaultScooters)
}

func getScooterHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var scoot *scooter = nil
	for _, candidate := range defaultScooters {
		if candidate.Id == id {
			scoot = &candidate
			break
		}
	}

	if scoot == nil {
		http.Error(w, "Scooter not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(scoot)
}

func patchScooterHandler(w http.ResponseWriter, r *http.Request) {
	user, err := doAuthStuff(&r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	index := -1
	for idx, candidate := range defaultScooters {
		if candidate.Id == id {
			index = idx
			break
		}
	}

	if index == -1 {
		http.Error(w, "Scooter not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var desiredValues scooterUpdate
	err = json.Unmarshal(body, &desiredValues)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if desiredValues.BatteryLevel != nil {
		newValue := *desiredValues.BatteryLevel
		if newValue > 100 || newValue < 0 {
			http.Error(w, "Field 'battery' must be in range [0, 100]", http.StatusBadRequest)
			return
		}
		defaultScooters[index].BatteryLevel = newValue
	}

	if desiredValues.Reserved != nil {
		if *desiredValues.Reserved {
			// trying to reserve
			if user.Reservation != nil {
				http.Error(
					w,
					fmt.Sprintf("user %s already has a scooter reserved", user.Name),
					http.StatusBadRequest,
				)
				return
			}

			if defaultScooters[index].Reserved {
				http.Error(w, "scooter is already reserved", http.StatusBadRequest)
				return
			}

			defaultScooters[index].Reserved = true
			user.Reservation = &defaultScooters[index].Id
			defaultUsers[user.Id] = user
		} else {
			// trying to release
			if user.Reservation == nil || *user.Reservation != defaultScooters[index].Id {
				http.Error(w, "user does not own reservation", http.StatusBadRequest)
				return
			}

			defaultScooters[index].Reserved = false
			user.Reservation = nil
			defaultUsers[user.Id] = user
		}
	}

	if desiredValues.Location != nil {
		err := checkLocation(*desiredValues.Location)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defaultScooters[index].Location = *desiredValues.Location
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(defaultScooters[index])
}

func main() {
	http.HandleFunc("GET /scooter", getScootersHandler)
	http.HandleFunc("GET /scooter/{id}", getScooterHandler)
	http.HandleFunc("PATCH /scooter/{id}", patchScooterHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
