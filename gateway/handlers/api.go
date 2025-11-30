package handlers

import (
	"encoding/json"
	"fmt"
	"gateway/database"
	"gateway/models"
	"gateway/services"
	"io"
	"net/http"
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
	// 1. Form Verilerini Al
	r.ParseMultipartForm(10 << 20) // 10MB limit

	name := r.FormValue("name")
	surname := r.FormValue("surname")

	// Frontend'den gelen dosya (muhtemelen 'blob' veya .webm uzantılı)
	file, _, err := r.FormFile("voice_record_file")
	if err != nil {
		http.Error(w, "Dosya alınamadı", 400)
		return
	}
	defer file.Close()

	// 2. WebM Dosyasını Belleğe Oku
	webmData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Dosya okunamadı", 500)
		return
	}

	// 3. Format Dönüşümü (WebM -> WAV)
	// Bu işlem sunucuda FFmpeg kurulu olmasını gerektirir.
	wavData, err := services.ConvertWebMToWav(webmData)
	if err != nil {
		fmt.Println("Dönüşüm Hatası:", err) // Logla
		http.Error(w, "Ses formatı dönüştürülemedi (FFmpeg hatası)", 500)
		return
	}

	// 4. Kullanıcıyı Veritabanına Kaydet
	// Dosyayı diske kaydetmediğimiz için VoicePath boş veya sembolik olabilir.
	user := models.User{
		Name:      name,
		Surname:   surname,
		VoicePath: "remote_stored",
	}

	if result := database.DB.Create(&user); result.Error != nil {
		http.Error(w, "Veritabanı hatası", 500)
		return
	}

	// 5. Analyze Servisine (Identificate) Gönder
	err = services.CallIdentificateService(user.ID, wavData)
	if err != nil {
		// Kullanıcı oluştu ama ses gönderilemedi.
		// Duruma göre DB'den silme işlemi (rollback) yapılabilir veya sadece hata dönülür.
		fmt.Printf("Analyze Service Hatası (User ID: %d): %v\n", user.ID, err)
		http.Error(w, "Kullanıcı oluşturuldu ancak analiz servisine gönderilemedi: "+err.Error(), 500)
		return
	}

	// Başarılı Yanıt
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"user_id": user.ID,
		"message": "Kullanıcı kaydedildi ve ses verisi işlendi.",
	})
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
