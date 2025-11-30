package models

// Settings
const (
	Port            = ":8080"
	SampleRate      = 16000
	PacketSize      = 640
	MinSegmentBytes = SampleRate * 2 * 3
	DBName          = "db.sqlite"
	RecordDir       = "record_matches"
)

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

type User struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Date    string `json:"date"`
}

type Record struct {
	ID       string   `json:"id"`
	Date     string   `json:"date"`
	Duration string   `json:"duration"`
	Topic    string   `json:"topic"`
	Speakers []string `json:"speakers"`
}
