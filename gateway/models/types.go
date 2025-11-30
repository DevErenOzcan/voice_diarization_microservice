package models

import "time"

// Settings
const (
	Port            = ":8080"
	SampleRate      = 16000
	PacketSize      = 640
	MinSegmentBytes = SampleRate * 2 * 3
	DBName          = "db.sqlite"
	RecordDir       = "record_matches"
)

// --- Veritabanı Modelleri (GORM) ---

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Surname   string    `json:"surname"`
	VoicePath string    `json:"voice_path"`
	CreatedAt time.Time `json:"date"` // GORM otomatik yönetir
}

type Record struct {
	ID        string    `gorm:"primaryKey" json:"id"` // Socket'ten gelen sessionID (string)
	Date      time.Time `json:"date"`
	Topic     string    `gorm:"default:'Genel'" json:"topic"`
	Sentiment string    `gorm:"default:'Nötr'" json:"-"`

	// İlişkiler (DB'de foreign key)
	Segments []Segment `gorm:"foreignKey:RecordID" json:"-"`

	// Hesaplanmış alanlar (DB'de sütunu yok)
	Duration string   `gorm:"-" json:"duration"`
	Speakers []string `gorm:"-" json:"speakers"`
}

type Segment struct {
	ID             uint    `gorm:"primaryKey" json:"-"`
	RecordID       string  `gorm:"index" json:"record_id"`
	StartOffset    float64 `json:"start"`
	EndOffset      float64 `json:"end"`
	Text           string  `json:"text"`
	TextSentiment  string  `json:"textSentiment"`
	VoiceSentiment string  `json:"voiceSentiment"`
	Speaker        string  `json:"speaker"`
}

// --- DTO (Data Transfer Objects) ---

// Frontend'e giden canlı analiz verisi
type LiveAnalysisResult struct {
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Text           string  `json:"text"`
	TextSentiment  string  `json:"textSentiment"`
	VoiceSentiment string  `json:"voiceSentiment"`
	Speaker        string  `json:"speaker"`
}

// Servisler arası iletişim payload'ı
type ServicePayload struct {
	RecordID       string           `json:"record_id,omitempty"`
	WavFile        []byte           `json:"wav_file,omitempty"`
	Text           string           `json:"text"`
	Start          float64          `json:"start"`
	End            float64          `json:"end"`
	Language       string           `json:"language"`
	TextSentiment  string           `json:"text_sentiment,omitempty"`
	VoiceSentiment string           `json:"voice_sentiment,omitempty"`
	Speaker        string           `json:"speaker,omitempty"`
	Segments       []ServicePayload `json:"segments,omitempty"`
}
