package database

import (
	"database/sql"
	_ "fmt"
	"log"
	"os"

	"gateway/models"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init() {
	if err := os.MkdirAll(models.RecordDir, 0755); err != nil {
		log.Fatal("Klasör oluşturma hatası:", err)
	}

	var err error
	DB, err = sql.Open("sqlite3", models.DBName)
	if err != nil {
		log.Fatal("DB Bağlantı hatası:", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS records (id TEXT PRIMARY KEY, date DATETIME, topic TEXT DEFAULT 'Genel', sentiment TEXT DEFAULT 'Nötr');`,
		`CREATE TABLE IF NOT EXISTS segments (id INTEGER PRIMARY KEY, record_id TEXT, start_offset REAL, end_offset REAL, text TEXT, text_sentiment TEXT, voice_sentiment TEXT, speaker TEXT);`,
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, surname TEXT, voice_path TEXT, created_at DATETIME);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatal("Tablo oluşturma hatası:", err)
		}
	}
	log.Println("Veritabanı hazır.")
}

// SQL sorgularını wrapper fonksiyonlara çevirebilirsin (Örnek):
func GetUsers() ([]models.User, error) {
	rows, err := DB.Query("SELECT id, name, surname, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Name, &u.Surname, &u.Date)
		users = append(users, u)
	}
	return users, nil
}
