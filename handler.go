package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Handler struct {
	db *sql.DB
}

func (handle Handler) GetScootersHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	rows, err := handle.db.Query("select * from scooters")
	if err != nil {
		http.Error(w, "no matching scooter found", http.StatusNotFound)
		return
	}
	defer rows.Close()

	var scoots []scooter
	for rows.Next() {
		var s scooter
		err = rows.Scan(&s.Id, &s.Reserved, &s.BatteryLevel, &s.Location.Latitude, &s.Location.Longitude)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		scoots = append(scoots, s)
	}

	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(scoots)
}

func (handle Handler) GetScooterHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var reserved bool
	var batteryLevel int
	var latitude float64
	var longitude float64
	err = handle.db.QueryRow("select * from scooters where id=?", id).Scan(&id, &reserved, &batteryLevel, &latitude, &longitude)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	scoot := scooter{
		Id: id,
		Reserved: reserved,
		BatteryLevel: batteryLevel,
		Location: location{Latitude: latitude, Longitude: longitude},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(scoot)
}

func (handle Handler) PatchScooterHandler(w http.ResponseWriter, r *http.Request) {
	user, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var reserved bool
	var batteryLevel int
	var latitude float64
	var longitude float64
	err = handle.db.QueryRow("select * from scooters where id=?", id).Scan(&id, &reserved, &batteryLevel, &latitude, &longitude)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	scoot := scooter{
		Id: id,
		Reserved: reserved,
		BatteryLevel: batteryLevel,
		Location: location{Latitude: latitude, Longitude: longitude},
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

	tx, err := handle.db.Begin()
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	reservedUp, err := tx.Prepare("UPDATE scooters SET reserved = ? WHERE id == ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	batteryUp, err := tx.Prepare("UPDATE scooters SET battery_level = ? WHERE id == ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	latitudeUp, err := tx.Prepare("UPDATE scooters SET latitude = ? WHERE id == ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	longitudeUp, err := tx.Prepare("UPDATE scooters SET longitude = ? WHERE id == ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	userUp, err := tx.Prepare("UPDATE users SET reservation = ? WHERE id == ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if desiredValues.BatteryLevel != nil {
		newValue := *desiredValues.BatteryLevel
		if newValue > 100 || newValue < 0 {
			http.Error(w, "Field 'battery' must be in range [0, 100]", http.StatusBadRequest)
			return
		}
		batteryUp.Exec(newValue, id)
		scoot.BatteryLevel = newValue
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

			if scoot.Reserved {
				http.Error(w, "scooter is already reserved", http.StatusBadRequest)
				return
			}

			log.Printf("Setting reserved=true for %s", id)
			reservedUp.Exec(true, id)
			scoot.Reserved = true
			userUp.Exec(id, user.Id)
		} else {
			// trying to release
			if user.Reservation == nil || *user.Reservation != scoot.Id {
				http.Error(w, "user does not own reservation", http.StatusBadRequest)
				return
			}

			log.Printf("Setting reserved=false for %s", id)
			reservedUp.Exec(false, id)
			scoot.Reserved = false
			userUp.Exec(nil, user.Id)
		}
	}

	if desiredValues.Location != nil {
		err := checkLocation(*desiredValues.Location)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		latitudeUp.Exec(desiredValues.Location.Latitude, id)
		longitudeUp.Exec("longitude", desiredValues.Location.Longitude, id)
		scoot.Location = *desiredValues.Location
	}

	log.Printf("Committing")
	err = tx.Commit()
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(scoot)
}
