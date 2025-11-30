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

	rows, err := database.DB.Query("SELECT id, name, surname, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		// Hata kontrolü eklenebilir
		rows.Scan(&u.ID, &u.Name, &u.Surname, &u.Date)
		users = append(users, u)
	}
	json.NewEncoder(w).Encode(users)
}

// POST /api/record_user
func HandleRecordUser(w http.ResponseWriter, r *http.Request) {
	// 10MB limit
	r.ParseMultipartForm(10 << 20)

	name := r.FormValue("name")
	surname := r.FormValue("surname")
	file, header, err := r.FormFile("voice_record_file")
	if err != nil {
		http.Error(w, "Dosya alınamadı", 400)
		return
	}
	defer file.Close()

	// Dosyayı diske kaydet
	filename := fmt.Sprintf("user_%d_%s%s", time.Now().Unix(), name, filepath.Ext(header.Filename))
	if filepath.Ext(header.Filename) == "" {
		filename += ".wav" // Varsayılan uzantı
	}
	fullPath := filepath.Join(models.RecordDir, filename)

	outFile, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, "Dosya kaydedilemedi", 500)
		return
	}
	defer outFile.Close() // Fonksiyon bitince dosyayı kapat

	if _, err := io.Copy(outFile, file); err != nil {
		http.Error(w, "Dosya yazılamadı", 500)
		return
	}

	// DB'ye kaydet
	_, err = database.DB.Exec("INSERT INTO users(name, surname, voice_path, created_at) values(?, ?, ?, ?)",
		name, surname, fullPath, time.Now().Format("2006-01-02 15:04:05"))

	if err != nil {
		http.Error(w, "Veritabanı hatası", 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GET /api/records
func HandleGetRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.DB.Query("SELECT id, date, topic FROM records ORDER BY date DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var records []models.Record
	for rows.Next() {
		var rec models.Record
		rows.Scan(&rec.ID, &rec.Date, &rec.Topic)

		// Süreyi hesapla (Orijinal mantık: En son segmentin bitiş zamanı)
		var maxEnd float64
		// Null gelme ihtimaline karşı sql.NullFloat64 kullanılabilir ama basitleştirmek için:
		err := database.DB.QueryRow("SELECT COALESCE(MAX(end_offset), 0) FROM segments WHERE record_id = ?", rec.ID).Scan(&maxEnd)
		if err != nil {
			maxEnd = 0
		}
		rec.Duration = fmt.Sprintf("%02d:%02d", int(maxEnd)/60, int(maxEnd)%60)

		// Konuşmacıları bul
		sRows, _ := database.DB.Query("SELECT DISTINCT speaker FROM segments WHERE record_id = ?", rec.ID)
		for sRows.Next() {
			var s string
			sRows.Scan(&s)
			rec.Speakers = append(rec.Speakers, s)
		}
		sRows.Close()

		if len(rec.Speakers) == 0 {
			rec.Speakers = []string{"Bilinmiyor"}
		}

		records = append(records, rec)
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

	rows, err := database.DB.Query(`SELECT start_offset, end_offset, text, speaker, text_sentiment, voice_sentiment 
						   FROM segments WHERE record_id = ? ORDER BY start_offset ASC`, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var segments []models.LiveAnalysisResult
	for rows.Next() {
		var s models.LiveAnalysisResult
		rows.Scan(&s.Start, &s.End, &s.Text, &s.Speaker, &s.TextSentiment, &s.VoiceSentiment)
		segments = append(segments, s)
	}

	// Eğer veri yoksa boş array dönmeli, null değil
	if segments == nil {
		segments = []models.LiveAnalysisResult{}
	}

	json.NewEncoder(w).Encode(segments)
}
