package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
)

const (
	SampleRate        = 16000
	BitDepth          = 16
	Channels          = 1
	PacketSize        = 640
	MinSegmentBytes   = SampleRate * 2 * 3 // En az 3 saniyelik konuşma
	WhisperServiceURL = "http://localhost:5000/"
	AnalyzeServiceURL = "http://localhost:5001/"
	BytesPerSecond    = SampleRate * (BitDepth / 8) * Channels
)

// --- Yapılar (Structs) ---

// Frontend'den gelen JSON komutları için
type IncomingJSONMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Frontend'e gönderilecek mesaj formatı
type WSResponse struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Frontend'in beklediği analiz sonucu formatı
type FrontendAnalysisResult struct {
	Start          float64 `json:"start"` // EKLENDİ
	End            float64 `json:"end"`   // EKLENDİ
	Text           string  `json:"text"`
	TextSentiment  string  `json:"textSentiment"`
	VoiceSentiment string  `json:"voiceSentiment"`
	Speaker        string  `json:"speaker"`
}

// Python Whisper servisinden dönen yanıt
type WhisperAPIResponse struct {
	Segments []AnalyzePayload `json:"segments"`
	Language string           `json:"language"`
}

// Analiz servisine gidecek ve oradan dönecek veri yapısı
type AnalyzePayload struct {
	SegmentID      string  `json:"segment_id,omitempty"`
	WavFile        []byte  `json:"wav_file,omitempty"`
	Text           string  `json:"text"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Language       string  `json:"language"`
	TextSentiment  string  `json:"text_sentiment,omitempty"`  // Backend'den gelen
	VoiceSentiment string  `json:"voice_sentiment,omitempty"` // Backend'den gelen
	Speaker        string  `json:"speaker,omitempty"`         // Backend'den gelen
}

// Thread-Safe WebSocket Bağlantısı
type ThreadSafeConn struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

// Güvenli yazma metodu
func (t *ThreadSafeConn) WriteJSON(v interface{}) error {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	return t.Conn.WriteJSON(v)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// --- Yardımcı Fonksiyonlar ---

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

// 3. Adım: Analiz Servisine Gönder ve Sonucu Frontend'e İlet
func sendToAnlyzeService(payload AnalyzePayload, wsConn *ThreadSafeConn) {
	// WavFile'ı JSON'a marshal ederken base64'e otomatik çevrilir
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

	// Analiz servisinden dönen veriyi parse et
	// (Servisin AnalyzePayload formatında zenginleştirilmiş veri döndüğünü varsayıyoruz)
	var analyzedResult AnalyzePayload
	if err := json.Unmarshal(body, &analyzedResult); err != nil {
		log.Printf("AnalyzeService yanıtı parse edilemedi: %v", err)
		return
	}

	// --- FRONTEND'E GÖNDERME KISMI (sendToAnlyzeService içinde) ---
	frontendPayload := FrontendAnalysisResult{
		Start:          analyzedResult.Start, // EKLENDİ
		End:            analyzedResult.End,   // EKLENDİ
		Text:           analyzedResult.Text,
		TextSentiment:  analyzedResult.TextSentiment,
		VoiceSentiment: analyzedResult.VoiceSentiment,
		Speaker:        analyzedResult.Speaker,
	}

	// WebSocket üzerinden React uygulamasına bas
	if err := wsConn.WriteJSON(WSResponse{
		Type:    "live_analysis",
		Payload: frontendPayload,
	}); err != nil {
		log.Printf("Frontend'e yazma hatası: %v", err)
	} else {
		log.Printf("Analiz sonucu frontend'e iletildi: %s", analyzedResult.Text)
	}
}

// 2. Adım: Whisper Servisine Gönder
func sendToWhisperxService(sessionID string, pcmData []byte, chunkIndex int, wsConn *ThreadSafeConn) {
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

	payloads := createAnalyzePayload(sessionID, pcmData, chunkIndex, apiResult)

	for _, payload := range payloads {
		// Analiz servisine gönderirken WebSocket bağlantısını da taşıyoruz
		go sendToAnlyzeService(payload, wsConn)
	}
}

func createAnalyzePayload(sessionID string, pcmData []byte, chunkIndex int, result WhisperAPIResponse) []AnalyzePayload {
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

		segment.SegmentID = fmt.Sprintf("%s_%d_%d", sessionID, chunkIndex, i)
		segment.WavFile = wavBuffer.Bytes()
		segment.Language = result.Language
		validSegments = append(validSegments, segment)
	}
	return validSegments
}

func handleJSONMessage(msg []byte, wsConn *ThreadSafeConn) {
	var req IncomingJSONMessage
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Println("JSON parse hatası:", err)
		return
	}

	switch req.Type {
	case "create_user":
		// React'taki Users.jsx'ten gelen { type: 'create_user', data: { name:..., surname:... } }
		log.Printf("Yeni kullanıcı isteği: %s", string(req.Data))
		// Burada veritabanına kayıt işlemi yapılabilir.
		// Şimdilik sadece logluyoruz ve başarılı yanıtı dönüyoruz.
		wsConn.WriteJSON(WSResponse{
			Type:    "notification",
			Payload: "Kullanıcı başarıyla oluşturuldu (Gateway Mock)",
		})

	case "get_users":
		// React tarafına kullanıcı listesini gönderme örneği
		log.Println("Kullanıcı listesi istendi")
		// Mock Data
		users := []map[string]string{
			{"id": "1", "name": "Test", "surname": "User", "date": "2025-01-01"},
		}
		wsConn.WriteJSON(WSResponse{
			Type:    "users_list",
			Payload: users,
		})
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Thread-safe bağlantı yapısını oluştur
	safeConn := &ThreadSafeConn{Conn: conn}
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	log.Printf("Oturum başladı: %s", sessionID)

	v, err := webrtcvad.New()
	if err != nil {
		log.Println("VAD Hatası:", err)
		return
	}
	v.SetMode(3)

	var (
		currentSegment []byte
		notSpeechCount = 0
		audioBuffer    = make([]byte, 0)
		chunkCounter   = 0
	)

	for {
		// Mesaj tipini okuyoruz (Binary mi, Text mi?)
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı koptu:", err)
			break
		}

		// EĞER MESAJ TEXT İSE (JSON KOMUTLARI)
		if messageType == websocket.TextMessage {
			go handleJSONMessage(message, safeConn)
			continue
		}

		// EĞER MESAJ BINARY İSE (SES VERİSİ)
		if messageType == websocket.BinaryMessage {
			audioBuffer = append(audioBuffer, message...)

			// VAD işlemleri (Senin kodun aynen korunuyor)
			for len(audioBuffer) >= PacketSize {
				frame := audioBuffer[:PacketSize]
				audioBuffer = audioBuffer[PacketSize:]

				isSpeaking, err := v.Process(SampleRate, frame)
				if err != nil {
					continue
				}

				if isSpeaking {
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

					// Frontend'e sonuç dönülebilmesi için safeConn iletiliyor
					go sendToWhisperxService(sessionID, segmentToSend, chunkCounter, safeConn)

					chunkCounter++
					currentSegment = make([]byte, 0)
					notSpeechCount = 0
				}
			}
		}
	}

	if len(currentSegment) > 0 {
		go sendToWhisperxService(sessionID, currentSegment, chunkCounter, safeConn)
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Gateway başlatıldı (8080)...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
