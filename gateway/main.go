package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os" // Dosya işlemleri için eklendi
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"github.com/maxhawkins/go-webrtcvad"
)

// ... Sabitler ...
const (
	SampleRate        = 16000
	BitDepth          = 16
	Channels          = 1
	PacketSize        = 640
	MinSegmentBytes   = SampleRate * 2 * 3
	WhisperServiceURL = "http://localhost:5000/"
	AnalyzeServiceURL = "http://localhost:5001/"
	BytesPerSecond    = SampleRate * (BitDepth / 8) * Channels
	DBName            = "db.sqlite"
	RecordDir         = "record_matches" // Kayıt klasörü
)

// ... Structlar ...

type IncomingJSONMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type WSResponse struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type CreateUserPayload struct {
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	AudioBase64 string `json:"audio_base64"`
}

type FrontendAnalysisResult struct {
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Text           string  `json:"text"`
	TextSentiment  string  `json:"textSentiment"`
	VoiceSentiment string  `json:"voiceSentiment"`
	Speaker        string  `json:"speaker"`
}

type WhisperAPIResponse struct {
	Segments []AnalyzePayload `json:"segments"`
	Language string           `json:"language"`
}

type AnalyzePayload struct {
	SegmentID      string  `json:"segment_id,omitempty"`
	RecordID       string  `json:"record_id,omitempty"`
	WavFile        []byte  `json:"wav_file,omitempty"`
	Text           string  `json:"text"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Language       string  `json:"language"`
	TextSentiment  string  `json:"text_sentiment,omitempty"`
	VoiceSentiment string  `json:"voice_sentiment,omitempty"`
	Speaker        string  `json:"speaker,omitempty"`
}

type DBRecord struct {
	ID        string   `json:"id"`
	Date      string   `json:"date"`
	Duration  string   `json:"duration"`
	Topic     string   `json:"topic"`
	Sentiment string   `json:"sentiment"`
	Speakers  []string `json:"speakers"`
}

type ThreadSafeConn struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

func (t *ThreadSafeConn) WriteJSON(v interface{}) error {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	return t.Conn.WriteJSON(v)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var httpClient = &http.Client{Timeout: 60 * time.Second}
var db *sql.DB

// --- VERİTABANI VE DOSYA SİSTEMİ İŞLEMLERİ ---

func initDB() {
	// 1. Klasör Kontrolü / Oluşturma
	if _, err := os.Stat(RecordDir); os.IsNotExist(err) {
		log.Printf("Klasör oluşturuluyor: %s", RecordDir)
		if err := os.MkdirAll(RecordDir, 0755); err != nil {
			log.Fatal("Klasör oluşturma hatası:", err)
		}
	}

	// 2. DB Bağlantısı
	var err error
	db, err = sql.Open("sqlite3", DBName)
	if err != nil {
		log.Fatal(err)
	}

	createRecordsTable := `
    CREATE TABLE IF NOT EXISTS records (
       id TEXT PRIMARY KEY,
       date DATETIME,
       topic TEXT DEFAULT 'Genel',
       sentiment TEXT DEFAULT 'Nötr'
    );`

	createSegmentsTable := `
    CREATE TABLE IF NOT EXISTS segments (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       record_id TEXT,
       start_offset REAL,
       end_offset REAL,
       text TEXT,
       text_sentiment TEXT,
       voice_sentiment TEXT,
       speaker TEXT,
       FOREIGN KEY(record_id) REFERENCES records(id)
    );`

	// DEĞİŞİKLİK: voice_sample (BLOB) yerine voice_path (TEXT) yaptık
	createUsersTable := `
    CREATE TABLE IF NOT EXISTS users (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       name TEXT,
       surname TEXT,
       voice_path TEXT, 
       created_at DATETIME
    );`

	if _, err := db.Exec(createRecordsTable); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(createSegmentsTable); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(createUsersTable); err != nil {
		log.Fatal(err)
	}

	log.Println("Veritabanı ve tablolar hazır.")
}

func saveRecord(id string) {
	stmt, err := db.Prepare("INSERT INTO records(id, date) values(?, ?)")
	if err != nil {
		log.Println("DB Prepare Error:", err)
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(id, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Println("Record kayıt hatası:", err)
	}
}

func saveSegment(p AnalyzePayload) {
	stmt, err := db.Prepare(`
       INSERT INTO segments(record_id, start_offset, end_offset, text, text_sentiment, voice_sentiment, speaker) 
       values(?, ?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		log.Println("DB Prepare Error:", err)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(p.RecordID, p.Start, p.End, p.Text, p.TextSentiment, p.VoiceSentiment, p.Speaker)
	if err != nil {
		log.Println("Segment kayıt hatası:", err)
	}
}

// --- API HANDLERS ---

func getRecordsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query("SELECT id, date, topic, sentiment FROM records ORDER BY date DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var records []DBRecord
	for rows.Next() {
		var r DBRecord
		if err := rows.Scan(&r.ID, &r.Date, &r.Topic, &r.Sentiment); err != nil {
			continue
		}

		var maxEnd float64
		_ = db.QueryRow("SELECT MAX(end_offset) FROM segments WHERE record_id = ?", r.ID).Scan(&maxEnd)

		r.Duration = "00:00"
		if maxEnd > 0 {
			mins := int(maxEnd) / 60
			secs := int(maxEnd) % 60
			r.Duration = fmt.Sprintf("%02d:%02d", mins, secs)
		}

		speakerRows, _ := db.Query("SELECT DISTINCT speaker FROM segments WHERE record_id = ?", r.ID)
		var speakers []string
		if speakerRows != nil {
			for speakerRows.Next() {
				var s string
				speakerRows.Scan(&s)
				if s != "" {
					speakers = append(speakers, s)
				}
			}
			speakerRows.Close()
		}
		r.Speakers = []string{"Bilinmiyor"}
		if len(speakers) > 0 {
			r.Speakers = speakers
		}

		records = append(records, r)
	}

	json.NewEncoder(w).Encode(records)
}

func getSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	recordID := r.URL.Query().Get("id")

	rows, err := db.Query("SELECT start_offset, end_offset, text, speaker, text_sentiment, voice_sentiment FROM segments WHERE record_id = ? ORDER BY start_offset ASC", recordID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var segments []FrontendAnalysisResult
	for rows.Next() {
		var s FrontendAnalysisResult
		rows.Scan(&s.Start, &s.End, &s.Text, &s.Speaker, &s.TextSentiment, &s.VoiceSentiment)
		segments = append(segments, s)
	}

	json.NewEncoder(w).Encode(segments)
}

// ... Yardımcı Fonksiyonlar ...

func writeWavHeader(w io.Writer, dataLength int) error {
	fileSize := dataLength + 36
	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, int32(fileSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, int32(16))
	binary.Write(buf, binary.LittleEndian, int16(1))
	binary.Write(buf, binary.LittleEndian, int16(Channels))
	binary.Write(buf, binary.LittleEndian, int32(SampleRate))
	binary.Write(buf, binary.LittleEndian, int32(SampleRate*Channels*BitDepth/8))
	binary.Write(buf, binary.LittleEndian, int16(Channels*BitDepth/8))
	binary.Write(buf, binary.LittleEndian, int16(BitDepth))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, int32(dataLength))
	_, err := w.Write(buf.Bytes())
	return err
}

func sendToAnlyzeService(payload AnalyzePayload, wsConn *ThreadSafeConn) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("JSON marshal hatası: %v", err)
		return
	}

	req, err := http.NewRequest("POST", AnalyzeServiceURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("AnalyzeService request hatası: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("AnalyzeService bağlantı hatası: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("AnalyzeService hata (%s): %s", resp.Status, string(body))
		return
	}

	var analyzedResult AnalyzePayload
	if err := json.Unmarshal(body, &analyzedResult); err != nil {
		log.Printf("AnalyzeService yanıtı parse edilemedi: %v", err)
		return
	}

	analyzedResult.RecordID = payload.RecordID
	analyzedResult.Start = payload.Start
	analyzedResult.End = payload.End

	saveSegment(analyzedResult)

	frontendPayload := FrontendAnalysisResult{
		Start:          analyzedResult.Start,
		End:            analyzedResult.End,
		Text:           analyzedResult.Text,
		TextSentiment:  analyzedResult.TextSentiment,
		VoiceSentiment: analyzedResult.VoiceSentiment,
		Speaker:        analyzedResult.Speaker,
	}

	if err := wsConn.WriteJSON(WSResponse{
		Type:    "live_analysis",
		Payload: frontendPayload,
	}); err != nil {
		log.Printf("Frontend'e yazma hatası: %v", err)
	} else {
		log.Printf("Analiz DB'ye yazıldı ve Frontend'e iletildi: %s", analyzedResult.Text)
	}
}

func sendToWhisperxService(sessionID string, pcmData []byte, chunkIndex int, wsConn *ThreadSafeConn, offsetSeconds float64) {
	req, err := http.NewRequest("POST", WhisperServiceURL, bytes.NewReader(pcmData))
	if err != nil {
		log.Printf("[%s] Request hatası: %v", sessionID, err)
		return
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[%s] Servis hatası: %v", sessionID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] Whisper hatası: %s", sessionID, resp.Status)
		return
	}

	var apiResult WhisperAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		log.Printf("[%s] JSON decode hatası: %v", sessionID, err)
		return
	}

	payloads := createAnalyzePayload(sessionID, pcmData, chunkIndex, apiResult, offsetSeconds)

	for _, payload := range payloads {
		go sendToAnlyzeService(payload, wsConn)
	}
}

func createAnalyzePayload(sessionID string, pcmData []byte, chunkIndex int, result WhisperAPIResponse, offsetSeconds float64) []AnalyzePayload {
	var validSegments []AnalyzePayload

	for i, segment := range result.Segments {
		startByte := int(segment.Start * float64(BytesPerSecond))
		endByte := int(segment.End * float64(BytesPerSecond))

		if startByte < 0 {
			startByte = 0
		}
		if endByte > len(pcmData) {
			endByte = len(pcmData)
		}
		if startByte >= endByte {
			continue
		}

		segmentPCM := pcmData[startByte:endByte]
		wavBuffer := new(bytes.Buffer)
		if err := writeWavHeader(wavBuffer, len(segmentPCM)); err == nil {
			wavBuffer.Write(segmentPCM)
		}

		segment.RecordID = sessionID
		segment.SegmentID = fmt.Sprintf("%s_%d_%d", sessionID, chunkIndex, i)
		segment.WavFile = wavBuffer.Bytes()
		segment.Language = result.Language
		segment.Start = segment.Start + offsetSeconds
		segment.End = segment.End + offsetSeconds

		validSegments = append(validSegments, segment)
	}
	return validSegments
}

// ... JSON Handler ...
func handleJSONMessage(msg []byte, wsConn *ThreadSafeConn) {
	var req IncomingJSONMessage
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Println("JSON parse hatası:", err)
		return
	}

	switch req.Type {
	case "create_user":
		// 1. Veriyi struct'a al
		var payload CreateUserPayload
		if err := json.Unmarshal(req.Data, &payload); err != nil {
			log.Printf("CreateUser payload hatası: %v", err)
			wsConn.WriteJSON(WSResponse{Type: "error", Payload: "Geçersiz veri formatı"})
			return
		}

		// 2. Base64 verisini çöz (Frontend'den gelen dosya verisi)
		fileBytes, err := base64.StdEncoding.DecodeString(payload.AudioBase64)
		if err != nil {
			log.Printf("Base64 decode hatası: %v", err)
			wsConn.WriteJSON(WSResponse{Type: "error", Payload: "Ses dosyası çözülemedi"})
			return
		}

		// 3. Dosyayı Diske Kaydet
		// Not: Tarayıcılar genelde WebM formatında kayıt yapar.
		// Bu yüzden uzantıyı .webm veriyoruz. (WAV header'ı EKLEMİYORUZ çünkü dosya zaten formatlı geliyor)
		// Frontend artık gerçek WAV gönderiyor, uzantıyı .wav yapıyoruz.
		filename := fmt.Sprintf("user_%d_%s.wav", time.Now().Unix(), payload.Name)
		filePath := filepath.Join(RecordDir, filename)

		// Direkt byte'ları yazıyoruz (Header ekleme yok!)
		if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
			log.Printf("Dosya yazma hatası: %v", err)
			wsConn.WriteJSON(WSResponse{Type: "error", Payload: "Dosya kaydedilemedi"})
			return
		}

		// 4. Veritabanına dosya yolunu kaydet
		stmt, err := db.Prepare("INSERT INTO users(name, surname, voice_path, created_at) values(?, ?, ?, ?)")
		if err != nil {
			log.Printf("DB Prepare hatası: %v", err)
			return
		}
		defer stmt.Close()

		res, err := stmt.Exec(payload.Name, payload.Surname, filePath, time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			log.Printf("DB Kayıt hatası: %v", err)
			wsConn.WriteJSON(WSResponse{Type: "error", Payload: "Veritabanı hatası"})
			return
		}

		lastID, _ := res.LastInsertId()
		log.Printf("Kullanıcı oluşturuldu: %s (Dosya: %s)", payload.Name, filePath)

		// Başarılı mesajı gönder
		wsConn.WriteJSON(WSResponse{
			Type:    "notification",
			Payload: fmt.Sprintf("Kullanıcı ve ses kaydı başarıyla oluşturuldu. (ID: %d)", lastID),
		})

	case "get_users":
		// ... (Burada değişiklik yok, önceki kodun aynısı) ...
		rows, err := db.Query("SELECT id, name, surname, created_at FROM users ORDER BY created_at DESC")
		if err != nil {
			log.Printf("Liste hatası: %v", err)
			return
		}
		defer rows.Close()

		var users []map[string]interface{}
		for rows.Next() {
			var id int
			var name, surname, createdAt string
			if err := rows.Scan(&id, &name, &surname, &createdAt); err != nil {
				continue
			}
			users = append(users, map[string]interface{}{
				"id":      id,
				"name":    name,
				"surname": surname,
				"date":    createdAt,
			})
		}
		wsConn.WriteJSON(WSResponse{Type: "users_list", Payload: users})
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	safeConn := &ThreadSafeConn{Conn: conn}
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	log.Printf("Oturum başladı: %s", sessionID)

	go saveRecord(sessionID)

	v, err := webrtcvad.New()
	if err != nil {
		log.Println("VAD Hatası:", err)
		return
	}
	v.SetMode(3)

	var (
		currentSegment      []byte
		notSpeechCount      = 0
		audioBuffer         = make([]byte, 0)
		chunkCounter        = 0
		totalBytesProcessed = 0
		segmentStartBytes   = 0
	)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı koptu:", err)
			break
		}

		if messageType == websocket.TextMessage {
			go handleJSONMessage(message, safeConn)
			continue
		}

		if messageType == websocket.BinaryMessage {
			audioBuffer = append(audioBuffer, message...)

			for len(audioBuffer) >= PacketSize {
				frame := audioBuffer[:PacketSize]
				currentFrameStartPos := totalBytesProcessed
				totalBytesProcessed += PacketSize
				audioBuffer = audioBuffer[PacketSize:]

				isSpeaking, err := v.Process(SampleRate, frame)
				if err != nil {
					continue
				}

				if isSpeaking {
					if len(currentSegment) == 0 {
						segmentStartBytes = currentFrameStartPos
					}
					notSpeechCount = 0
					currentSegment = append(currentSegment, frame...)
				} else {
					notSpeechCount++
					if len(currentSegment) > 0 {
						currentSegment = append(currentSegment, frame...)
					}
				}

				if notSpeechCount > 25 && len(currentSegment) > MinSegmentBytes {
					segmentToSend := make([]byte, len(currentSegment))
					copy(segmentToSend, currentSegment)
					offsetSeconds := float64(segmentStartBytes) / float64(BytesPerSecond)

					go sendToWhisperxService(sessionID, segmentToSend, chunkCounter, safeConn, offsetSeconds)

					chunkCounter++
					currentSegment = make([]byte, 0)
					notSpeechCount = 0
				}
			}
		}
	}
}

func main() {
	initDB()

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/api/records", getRecordsHandler)
	http.HandleFunc("/api/segments", getSegmentsHandler)

	log.Println("Gateway başlatıldı (8080)...")
	log.Println("API Endpoints: /api/records, /api/segments")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
