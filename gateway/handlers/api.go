package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gateway/database"
	"gateway/models"
)

// GET /api/users
func HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var users []models.User
	// CreatedAt tarihine göre tersten sırala ve getir
	if err := database.DB.Order("created_at desc").Find(&users).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(users)
}

// POST /api/record_user
func HandleRecordUser(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20) // 10MB limit

	name := r.FormValue("name")
	surname := r.FormValue("surname")
	file, header, err := r.FormFile("voice_record_file")
	if err != nil {
		http.Error(w, "Dosya alınamadı", 400)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("user_%d_%s%s", time.Now().Unix(), name, filepath.Ext(header.Filename))
	if filepath.Ext(header.Filename) == "" {
		filename += ".wav"
	}
	fullPath := filepath.Join(models.RecordDir, filename)

	outFile, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, "Dosya kaydedilemedi", 500)
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		http.Error(w, "Dosya yazılamadı", 500)
		return
	}

	// GORM ile Kaydet
	user := models.User{
		Name:      name,
		Surname:   surname,
		VoicePath: fullPath,
		// CreatedAt GORM tarafından otomatik set edilir
	}

	if result := database.DB.Create(&user); result.Error != nil {
		http.Error(w, "Veritabanı hatası", 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GET /api/records
func HandleGetRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var records []models.Record
	// Kayıtları tarihe göre sırala
	if err := database.DB.Order("date desc").Find(&records).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Her kayıt için ek bilgileri (Süre ve Konuşmacılar) hesapla
	for i := range records {
		var maxEnd float64
		// En son segmentin bitiş zamanını bul
		database.DB.Model(&models.Segment{}).
			Where("record_id = ?", records[i].ID).
			Select("COALESCE(MAX(end_offset), 0)").
			Scan(&maxEnd)

		records[i].Duration = fmt.Sprintf("%02d:%02d", int(maxEnd)/60, int(maxEnd)%60)

		// Konuşmacıları bul (Tekrarsız)
		var speakers []string
		database.DB.Model(&models.Segment{}).
			Distinct("speaker").
			Where("record_id = ?", records[i].ID).
			Pluck("speaker", &speakers)

		if len(speakers) == 0 {
			records[i].Speakers = []string{"Bilinmiyor"}
		} else {
			records[i].Speakers = speakers
		}
	}

	json.NewEncoder(w).Encode(records)
}

// GET /api/segments?id=...
func HandleGetSegments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")

	if id == "" {
		http.Error(w, "id parametresi gerekli", 400)
		return
	}

	var segments []models.Segment
	err := database.DB.Where("record_id = ?", id).
		Order("start_offset asc").
		Find(&segments).Error

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Segment modelini frontend'in beklediği LiveAnalysisResult formatına çevir
	var results []models.LiveAnalysisResult
	for _, s := range segments {
		results = append(results, models.LiveAnalysisResult{
			Start:          s.StartOffset,
			End:            s.EndOffset,
			Text:           s.Text,
			Speaker:        s.Speaker,
			TextSentiment:  s.TextSentiment,
			VoiceSentiment: s.VoiceSentiment,
		})
	}

	if results == nil {
		results = []models.LiveAnalysisResult{}
	}

	json.NewEncoder(w).Encode(results)
}
