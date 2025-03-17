package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
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

	queryParams := r.URL.Query()
	var rows *sql.Rows
	if queryParams.Get("user") != "" {
		scoot, err := getScooterByUser(queryParams.Get("user"), handle.db)
		if err != nil {
			http.Error(w, "no matching scooter found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "    ")
		encoder.Encode([1]scooterApi{scoot})
		return
	} else {
		selectSmt := "select scoot.*, reso.active from scooters scoot left join reservations reso on reso.scooter_id = scoot.id;"
		rows, err = handle.db.Query(selectSmt)

		if err != nil {
			http.Error(w, "no matching scooter found", http.StatusNotFound)
			return
		}
		defer rows.Close()
	
		var scoots []scooterApi
		for rows.Next() {
			var reserved sql.NullBool
			var scoot scooterApi
			scoot.Id.Defined = true
			scoot.BatteryLevel.Defined = true
			scoot.Location.Defined = true
			err = rows.Scan(&scoot.Id.Value, &scoot.BatteryLevel.Value, &scoot.Location.Value.Latitude, &scoot.Location.Value.Longitude, &reserved)
			if err != nil {
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			scoot.Reserved = Optional[bool]{Defined: true, Value: reserved.Bool}
			scoots = append(scoots, scoot)
		}
	
		w.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "    ")
		encoder.Encode(scoots)
	}
}

func getScooterByUser(userId string, db *sql.DB) (scooterApi, error) {
	var s scooterApi
	query := `
		select scoot.*
		from scooters scoot
		join reservations reso on scoot.id = reso.scooter_id
		where reso.user_id = ?
	`
	err := db.QueryRow(query, userId).Scan(&s.Id.Value, &s.BatteryLevel.Value, &s.Location.Value.Latitude, &s.Location.Value.Longitude)
	if err != nil {
		return scooterApi{}, err
	}
	s.Id.Defined = true
	s.BatteryLevel.Defined = true
	s.Location.Defined = true
	s.Reserved = Optional[bool]{Defined: true, Value: true}
	return s, nil
}

func (handle Handler) GetScooterHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var reserved sql.NullBool
	var scoot scooterApi
	err = handle.db.QueryRow("select scoot.*, reso.active from scooters scoot left join reservations reso on reso.scooter_id = scoot.id where id=?", id).Scan(&scoot.Id.Value, &scoot.BatteryLevel.Value, &scoot.Location.Value.Latitude, &scoot.Location.Value.Longitude, &reserved)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	scoot.Id.Defined = true
	scoot.BatteryLevel.Defined = true
	scoot.Location.Defined = true
	scoot.Reserved = Optional[bool]{Defined: true, Value: reserved.Bool}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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

	var batteryLevel int
	var latitude float64
	var longitude float64
	var reserved sql.NullBool
	var user_id sql.NullString
	err = handle.db.QueryRow("select scoot.*, reso.active, reso.user_id from scooters scoot left join reservations reso on reso.scooter_id = scoot.id where id=?", id).Scan(&id, &batteryLevel, &latitude, &longitude, &reserved, &user_id)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	scoot := scooter{
		Id: id,
		BatteryLevel: batteryLevel,
		Location: location{Latitude: latitude, Longitude: longitude},
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var desiredValues scooterApi
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
	reservationCreate, err := tx.Prepare("INSERT INTO reservations (scooter_id, user_id, start_time, active) VALUES (?, ?, ?, true)")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	reservationEnd, err := tx.Prepare("UPDATE reservations SET end_time = ?, active = false WHERE scooter_id = ?")
	if err != nil {
		log.Printf("Prepare: %s", err.Error())
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if desiredValues.BatteryLevel.Defined {
		newValue := desiredValues.BatteryLevel.Value
		if newValue > 100 || newValue < 0 {
			http.Error(w, "Field 'battery' must be in range [0, 100]", http.StatusBadRequest)
			return
		}
		batteryUp.Exec(newValue, id)
		scoot.BatteryLevel = newValue
	}

	if desiredValues.Reserved.Defined {
		if desiredValues.Reserved.Value {
			// trying to reserve
			if reserved.Valid && reserved.Bool {
				http.Error(
					w,
					fmt.Sprintf("user %s already has a scooter reserved", user.Name),
					http.StatusBadRequest,
				)
				return
			}

			log.Printf("Reserving scooter %s for %s", id, user.Id)
			now := time.Now().UTC().Unix()
			reservationCreate.Exec(id, user.Id, now)
		} else {
			// trying to release
			if !reserved.Valid || !reserved.Bool {
				http.Error(w, "scooter is not reserved", http.StatusBadRequest)
				return
			}
			if !user_id.Valid || user_id.String != user.Id {
				http.Error(w, "user does not own reservation", http.StatusBadRequest)
				return
			}

			now := time.Now().UTC().Unix()
			log.Printf("Ending reservation for %s", id)
			reservationEnd.Exec(now, id)
		}
	}

	if desiredValues.Location.Defined {
		err := checkLocation(desiredValues.Location.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		latitudeUp.Exec(desiredValues.Location.Value.Latitude, id)
		longitudeUp.Exec("longitude", desiredValues.Location.Value.Longitude, id)
		scoot.Location = desiredValues.Location.Value
	}

	log.Printf("Committing")
	err = tx.Commit()
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	outScoot := scooterApi{
		Id: Optional[string]{Defined: true, Value: scoot.Id},
		BatteryLevel: Optional[int]{Defined: true, Value: scoot.BatteryLevel},
		Location: Optional[location]{Defined: true, Value: scoot.Location},
	}
	if desiredValues.Reserved.Defined {
		outScoot.Reserved = Optional[bool]{Defined: true, Value: desiredValues.Reserved.Value}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(outScoot)
}

func (handle *Handler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	user, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	if user.Name != id {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	outUser := userApi{
		Id: Optional[string]{Defined: true, Value: user.Id},
		Name: Optional[string]{Defined: true, Value: user.Name},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(outUser)
}
