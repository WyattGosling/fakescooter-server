package main

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func doAuthStuff(header *http.Header, db *sql.DB) (user, error) {
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
	foundUser := user{}
	err = db.QueryRow("select * from users where name = ?", username).Scan(&foundUser.Id, &foundUser.Name, &foundUser.Reservation)
	if err != nil {
		log.Printf("doAuthStuff: %s", err.Error())
		return user{}, errors.New("unknown user")
	}

	return foundUser, nil
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

func makeDb() (*sql.DB, error) {
	os.Remove("./scooters.db")
	db, err := sql.Open("sqlite3", "./scooters.db")
	if err != nil {
		return nil, err
	}

	createScootersTable := `
	create table scooters (id text not null primary key, reserved integer, battery_level integer, latitude real, longitude real);
	delete from scooters;
	`
	createUsersTable := `
	create table users (id text not null primary key, name text, reservation text);
	delete from users;
	`

	_, err = db.Exec(createScootersTable)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(createUsersTable)
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	scooterInserter, err := tx.Prepare("insert into scooters(id, reserved, battery_level, latitude, longitude) values (?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer scooterInserter.Close()

	userInserter, err := tx.Prepare("insert into users(id, name, reservation) values (?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer userInserter.Close()

	_, err = scooterInserter.Exec("abc123", false, 99, 49.26227, -123.14242)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("def456", false, 88, 49.26636, -123.14226)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("ghi789", true, 77, 49.26532, -123.13659)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("jkl012", false, 9, 49.26443, -123.13469)
	if err != nil {
		return nil, err
	}

	_, err = userInserter.Exec("a1", "pay2go", nil)
	if err != nil {
		return nil, err
	}
	_, err = userInserter.Exec("b2", "basic", nil)
	if err != nil {
		return nil, err
	}
	_, err = userInserter.Exec("c3", "premium", nil)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	db, err := makeDb()
	if err != nil {
		log.Printf("Database setup failure: %s", err.Error())
		return
	}
	defer db.Close()

	handle := Handler{db: db}
	http.HandleFunc("GET /scooter", handle.GetScootersHandler)
	http.HandleFunc("GET /scooter/{id}", handle.GetScooterHandler)
	http.HandleFunc("PATCH /scooter/{id}", handle.PatchScooterHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
