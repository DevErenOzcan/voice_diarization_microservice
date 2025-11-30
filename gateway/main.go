package main

import (
	"log"
	"net/http"

	"gateway/database"
	"gateway/handlers"
	"gateway/models"
)

func main() {
	// 1. Veritabanını Başlat (GORM)
	database.Init()

	// 2. Rotaları Tanımla
	// WebSocket
	http.HandleFunc("/ws", handlers.HandleLiveAudio)

	// REST API
	http.HandleFunc("/api/users", handlers.HandleGetUsers)
	http.HandleFunc("/api/record_user", handlers.HandleRecordUser)
	http.HandleFunc("/api/records", handlers.HandleGetRecords)
	http.HandleFunc("/api/segments", handlers.HandleGetSegments)

	// 3. Sunucuyu Başlat
	log.Printf("Gateway başlatıldı: %s", models.Port)
	log.Fatal(http.ListenAndServe(models.Port, nil))
}
