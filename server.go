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
	err = db.QueryRow("select * from users where name = ?", username).Scan(&foundUser.Id, &foundUser.Name)
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
	create table scooters (id text not null primary key, battery_level integer, latitude real, longitude real);
	delete from scooters;
	`
	createUsersTable := `
	create table users (id text not null primary key, name text);
	delete from users;
	`

	createReservationsTable := `
	create table reservations (
		scooter_id text not null,
		user_id text not null,
		start_time integer not null,
		end_time integer,
		active boolean,
		foreign key (scooter_id) references scooters(id),
		foreign key (user_id) references users(id)
	);
	delete from reservations;`

	_, err = db.Exec(createScootersTable)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(createUsersTable)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(createReservationsTable)
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	scooterInserter, err := tx.Prepare("insert into scooters(id, battery_level, latitude, longitude) values (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer scooterInserter.Close()

	userInserter, err := tx.Prepare("insert into users(id, name) values (?, ?)")
	if err != nil {
		return nil, err
	}
	defer userInserter.Close()

	reservationInserter, err := tx.Prepare("insert into reservations(scooter_id, user_id, start_time, end_time, active) values (?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer reservationInserter.Close()

	_, err = scooterInserter.Exec("abc123", 99, 49.26227, -123.14242)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("def456", 88, 49.26636, -123.14226)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("ghi789", 77, 49.26532, -123.13659)
	if err != nil {
		return nil, err
	}
	_, err = scooterInserter.Exec("jkl012", 9, 49.26443, -123.13469)
	if err != nil {
		return nil, err
	}

	_, err = userInserter.Exec("a1", "pay2go")
	if err != nil {
		return nil, err
	}
	_, err = userInserter.Exec("b2", "basic")
	if err != nil {
		return nil, err
	}
	_, err = userInserter.Exec("c3", "premium")
	if err != nil {
		return nil, err
	}

	_, err = reservationInserter.Exec("ghi789", "c3", 1742182920, nil, true)
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
	http.HandleFunc("GET /user/{id}", handle.GetUserHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
