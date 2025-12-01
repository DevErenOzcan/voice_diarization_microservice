package database

import (
	"gateway/models"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() {
	var err error
	// GORM ile SQLite bağlantısı
	DB, err = gorm.Open(sqlite.Open(models.DBName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // Sadece hataları logla, konsolu kirletme
	})
	if err != nil {
		log.Fatal("DB Bağlantı hatası:", err)
	}

	// Tabloları otomatik oluştur veya güncelle
	err = DB.AutoMigrate(&models.User{}, &models.Record{}, &models.Segment{})
	if err != nil {
		log.Fatal("Migrasyon hatası:", err)
	}

	log.Println("Veritabanı hazır (GORM).")
}
