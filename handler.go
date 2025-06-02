package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	db *sql.DB
}

func (handle Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "no authorization provided", http.StatusUnauthorized)
		return
	}

	// assuming Basic auth
	bytes, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		http.Error(w, "invalid authorization format", http.StatusUnauthorized)
		return
	}

	authStr := string(bytes)
	log.Printf("Auth string: %s", authStr)
	splits := strings.Split(authStr, ":")
	username := splits[0]
	password := splits[1]

	var user user
	err = handle.db.QueryRow("select id, name from users where name = ? and password = ?", username, password).Scan(&user.Id, &user.Name)
	if err != nil {
		log.Printf("Login Error: %s", err.Error())
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	log.Printf("User %s logged in", user.Name)

	token := generateRandomString(32, 64)
	expiresAt := time.Now().Add(time.Hour).Unix()
	_, err = handle.db.Exec("insert into tokens (user_id, token, expires_at) values (?, ?, ?)", user.Id, token, expiresAt)
	if err != nil {
		log.Printf("Error inserting token: %s", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer " + token)
	w.WriteHeader(http.StatusOK)
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
		if !scoot.Reservation.Active {
			http.Error(w, "no matching scooter found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "    ")
		encoder.Encode([1]scooterOut{scoot})
		return
	} else {
		selectSmt := "select scoot.*, reso.active, reso.start_time from scooters scoot left join reservations reso on reso.scooter_id = scoot.id;"
		rows, err = handle.db.Query(selectSmt)

		if err != nil {
			http.Error(w, "no matching scooter found", http.StatusNotFound)
			return
		}
		defer rows.Close()
	
		var scoots []scooterOut
		for rows.Next() {
			var scoot scooterOut
			var reso reservation
			err = rows.Scan(&scoot.Id, &scoot.BatteryLevel, &scoot.Location.Latitude, &scoot.Location.Longitude, &reso.Active, &reso.StartTime)
			if err != nil {
				log.Printf("Scan Error: %s", err.Error())
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			if reso.Active.Valid {
				scoot.Reservation.Active = reso.Active.Bool
				scoot.Reservation.StartTime = reso.StartTime.Int64
			}
			scoots = append(scoots, scoot)
		}
	
		w.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "    ")
		encoder.Encode(scoots)
	}
}

func getScooterByUser(userId string, db *sql.DB) (scooterOut, error) {
	var s scooterOut
	var r reservationOut
	query := `
		select scoot.*, reso.active, reso.start_time
		from scooters scoot
		join reservations reso on scoot.id = reso.scooter_id
		where reso.user_id = ?
	`
	err := db.QueryRow(query, userId).Scan(&s.Id, &s.BatteryLevel, &s.Location.Latitude, &s.Location.Longitude, &r.Active, &r.StartTime)
	if err != nil {
		return scooterOut{}, err
	}
	s.Reservation = r
	return s, nil
}

func (handle Handler) GetScooterHandler(w http.ResponseWriter, r *http.Request) {
	_, err := doAuthStuff(&r.Header, handle.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var scoot scooterOut
	var reso reservation
	err = handle.db.QueryRow("select scoot.*, reso.active, reso.start_time from scooters scoot left join reservations reso on reso.scooter_id = scoot.id where id=?", id).Scan(&scoot.Id, &scoot.BatteryLevel, &scoot.Location.Latitude, &scoot.Location.Longitude, &reso.Active, &reso.StartTime)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if reso.Active.Valid {
		scoot.Reservation.Active = reso.Active.Bool
		scoot.Reservation.StartTime = reso.StartTime.Int64
	}

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
	err = handle.db.QueryRow("select scoot.*, reso.active, reso.user_id from scooters scoot left join reservations reso on reso.scooter_id = scoot.id and reso.active = true where id=?", id).Scan(&id, &batteryLevel, &latitude, &longitude, &reserved, &user_id)
	if err == sql.ErrNoRows {
		http.Error(w, "No matching scooter found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	inScoot := scooter{
		Id: id,
		BatteryLevel: batteryLevel,
		Location: location{Latitude: latitude, Longitude: longitude},
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var desiredValues scooterIn
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
		inScoot.BatteryLevel = newValue
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
		inScoot.Location = desiredValues.Location.Value
	}

	log.Printf("Committing")
	err = tx.Commit()
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	outScoot := scooterOut{
		Id: inScoot.Id,
		BatteryLevel: inScoot.BatteryLevel,
		Location: inScoot.Location,
	}

	var reservation reservationOut
	handle.db.QueryRow("select active, start_time from reservations where scooter_id=? and active=1", id).Scan(&reservation.Active, &reservation.StartTime)
	outScoot.Reservation = reservation

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

	outUser := userOut{
		Id: Optional[string]{Defined: true, Value: user.Id},
		Name: Optional[string]{Defined: true, Value: user.Name},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	encoder.Encode(outUser)
}


const asciiChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateRandomString(minLen, maxLen int) string {
	length := rand.Intn(maxLen-minLen+1) + minLen
	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = asciiChars[rand.Intn(len(asciiChars))]
	}
	return string(bytes)
}
