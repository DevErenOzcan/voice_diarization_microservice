package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync" // WaitGroup için gerekli
	"time"

	"gateway/database"
	"gateway/models"
	"gateway/services"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleLiveAudio(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WS Upgrade Error:", err)
		return
	}
	// Bağlantıyı fonksiyon sonunda kapat, ancak önce WaitGroup'u bekle
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%d", time.Now().Unix())
	log.Printf("Canlı analiz başladı: %s", sessionID)

	// Session kaydını oluştur
	newRecord := models.Record{
		ID:   sessionID,
		Date: time.Now(),
	}
	database.DB.Create(&newRecord)

	vad, _ := webrtcvad.New()
	vad.SetMode(3)

	var (
		audioBuffer    []byte
		currentSegment []byte
		isSpeaking     bool
		silenceCounter int
		bytesProcessed int
		wg             sync.WaitGroup // Asenkron işlemler için bekleme grubu
	)

	connLock := &sync.Mutex{}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı kesildi veya hata:", err)
			break
		}

		// 1. "STOP" mesajı kontrolü (Frontend durdur butonuna bastığında)
		if msgType == websocket.TextMessage && string(data) == "STOP" {
			log.Println("Durdurma isteği alındı, tampon temizleniyor...")

			// Eğer elde işlenmemiş segment varsa, onu zorla işle
			if len(currentSegment) > 0 {
				segmentCopy := make([]byte, len(currentSegment))
				copy(segmentCopy, currentSegment)
				offsetSec := float64(bytesProcessed-len(currentSegment)) / float64(models.SampleRate*2)

				wg.Add(1) // Bekleme grubuna ekle
				go processAndRespond(sessionID, segmentCopy, offsetSec, conn, connLock, &wg)
			}
			break // Döngüden çık, aşağıdaki wg.Wait() çalışsın
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		audioBuffer = append(audioBuffer, data...)

		for len(audioBuffer) >= models.PacketSize {
			frame := audioBuffer[:models.PacketSize]
			frameStart := bytesProcessed
			bytesProcessed += models.PacketSize
			audioBuffer = audioBuffer[models.PacketSize:]

			active, err := vad.Process(models.SampleRate, frame)
			if err != nil {
				continue
			}

			if active {
				isSpeaking = true
				silenceCounter = 0
				currentSegment = append(currentSegment, frame...)
			} else {
				silenceCounter++
				if isSpeaking {
					currentSegment = append(currentSegment, frame...)
				}
			}

			if silenceCounter > 25 && len(currentSegment) > models.MinSegmentBytes {
				segmentCopy := make([]byte, len(currentSegment))
				copy(segmentCopy, currentSegment)
				offsetSec := float64(frameStart-len(currentSegment)) / float64(models.SampleRate*2)

				// Go Routine içinde servislere gönder
				wg.Add(1) // Bekleme grubuna ekle
				go processAndRespond(sessionID, segmentCopy, offsetSec, conn, connLock, &wg)

				currentSegment = nil
				isSpeaking = false
				silenceCounter = 0
			}
		}
	}

	// Döngü bittiğinde (STOP geldiğinde), tüm asenkron işlemlerin bitmesini bekle
	wg.Wait()
	log.Println("Analiz oturumu sonlandırıldı:", sessionID)
}

// processAndRespond güncellendi: ID -> İsim Soyisim dönüşümü eklendi
func processAndRespond(recordID string, pcmData []byte, offset float64, conn *websocket.Conn, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done() // İşlem bitince WaitGroup'tan düş

	whisperResp, err := services.CallWhisperService(pcmData)
	if err != nil {
		log.Println("Whisper Error:", err)
		return
	}

	for _, seg := range whisperResp.Segments {
		wavData := services.CreateWav(pcmData)
		payload := models.ServicePayload{
			RecordID: recordID,
			Text:     seg.Text,
			WavFile:  wavData,
			Language: whisperResp.Language,
		}

		analyzeResp, err := services.CallAnalyzeService(payload)
		if err != nil {
			log.Println("Analyze Error:", err)
			continue
		}

		// --- DEĞİŞİKLİK BURADA BAŞLIYOR ---
		// Gelen speaker ID'si (örn: "1" veya "Unknown") üzerinden ismi bul
		displaySpeaker := analyzeResp.Speaker // Varsayılan olarak ID kalsın

		if analyzeResp.Speaker != "Unknown" && analyzeResp.Speaker != "" {
			var user models.User
			// Veritabanında ID'ye göre kullanıcıyı ara
			// analyzeResp.Speaker string olduğu için, veritabanı sorgusunda ID ile eşleşmesi gerekir.
			if result := database.DB.First(&user, "id = ?", analyzeResp.Speaker); result.Error == nil {
				// Kullanıcı bulunduysa format: "Ad Soyad"
				displaySpeaker = fmt.Sprintf("%s %s", user.Name, user.Surname)
			}
		}
		// --- DEĞİŞİKLİK BURADA BİTİYOR ---

		finalStart := offset + seg.Start
		finalEnd := offset + seg.End

		// Veritabanına segmenti kaydet
		// Speaker alanına artık Ad Soyad yazıyoruz
		newSegment := models.Segment{
			RecordID:        recordID,
			StartOffset:     finalStart,
			EndOffset:       finalEnd,
			Text:            analyzeResp.Text,
			TextSentiment:   analyzeResp.TextSentiment,
			VoiceSentiment:  analyzeResp.VoiceSentiment,
			Speaker:         displaySpeaker, // ID yerine isim
			SimilarityScore: analyzeResp.SimilarityScore,
		}
		database.DB.Create(&newSegment)

		response := map[string]interface{}{
			"type": "live_analysis",
			"payload": models.LiveAnalysisResult{
				Start:           finalStart,
				End:             finalEnd,
				Text:            analyzeResp.Text,
				TextSentiment:   analyzeResp.TextSentiment,
				VoiceSentiment:  analyzeResp.VoiceSentiment,
				Speaker:         displaySpeaker, // ID yerine isim frontend'e gidiyor
				SimilarityScore: analyzeResp.SimilarityScore,
			},
		}

		mu.Lock()
		// Bağlantı kapanmadan yazmaya çalış
		if err := conn.WriteJSON(response); err != nil {
			log.Println("WS Write Error (muhtemelen bağlantı kapandı):", err)
		}
		mu.Unlock()
	}
}
