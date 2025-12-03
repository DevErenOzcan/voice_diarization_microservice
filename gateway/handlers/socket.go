package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync"
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
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%d", time.Now().Unix())
	log.Printf("Canlı analiz başladı: %s", sessionID)

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
		wg             sync.WaitGroup
	)

	connLock := &sync.Mutex{}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı kesildi veya hata:", err)
			break
		}

		if msgType == websocket.TextMessage && string(data) == "STOP" {
			log.Println("Durdurma isteği alındı, tampon temizleniyor...")
			if len(currentSegment) > 0 {
				segmentCopy := make([]byte, len(currentSegment))
				copy(segmentCopy, currentSegment)
				offsetSec := float64(bytesProcessed-len(currentSegment)) / float64(models.SampleRate*2)

				wg.Add(1)
				go processAndRespond(sessionID, segmentCopy, offsetSec, conn, connLock, &wg)
			}
			break
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

				wg.Add(1)
				go processAndRespond(sessionID, segmentCopy, offsetSec, conn, connLock, &wg)

				currentSegment = nil
				isSpeaking = false
				silenceCounter = 0
			}
		}
	}

	wg.Wait()
	log.Println("Analiz oturumu sonlandırıldı:", sessionID)
}

// processAndRespond: Split (ayrılmış) mimariye göre güncellendi
func processAndRespond(recordID string, pcmData []byte, offset float64, conn *websocket.Conn, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Whisper Servisi: Sesi Metne Çevir
	whisperResp, err := services.CallWhisperService(pcmData)
	if err != nil {
		log.Println("Whisper Error:", err)
		return
	}

	for _, seg := range whisperResp.Segments {
		// WAV oluştur (Audio servisi wav formatı bekler)
		wavData := services.CreateWav(pcmData)

		// 2. Text Service: Metin Duygu Analizi (Bağımsız Çağrı)
		textSentiment, err := services.CallTextSentimentService(seg.Text)
		if err != nil {
			log.Printf("Text Sentiment Error (Text: %s): %v", seg.Text, err)
			textSentiment = "Nötr" // Hata durumunda varsayılan
		}

		// 3. Audio Service: Ses Duygusu ve Konuşmacı Tanıma (Bağımsız Çağrı)
		audioPayload := models.ServicePayload{WavFile: wavData}
		audioResp, err := services.CallAudioAnalyzeService(audioPayload)
		if err != nil {
			log.Println("Audio Analyze Error:", err)
			// Hata durumunda varsayılan değerler
			audioResp = models.ServicePayload{
				VoiceSentiment:  "Bilinmiyor",
				Speaker:         "Unknown",
				SimilarityScore: 0.0,
			}
		}

		// 4. Konuşmacı ID'sini İsim Soyisime Çevirme (Gateway'in görevi)
		displaySpeaker := audioResp.Speaker // Varsayılan olarak ID kalsın (örn: "1")
		if audioResp.Speaker != "Unknown" && audioResp.Speaker != "" {
			var user models.User
			// Veritabanında ID'ye göre kullanıcıyı ara
			if result := database.DB.First(&user, "id = ?", audioResp.Speaker); result.Error == nil {
				displaySpeaker = fmt.Sprintf("%s %s", user.Name, user.Surname)
			}
		}

		finalStart := offset + seg.Start
		finalEnd := offset + seg.End

		// Veritabanına segmenti kaydet
		newSegment := models.Segment{
			RecordID:        recordID,
			StartOffset:     finalStart,
			EndOffset:       finalEnd,
			Text:            seg.Text,
			TextSentiment:   textSentiment,            // Text servisinden geldi
			VoiceSentiment:  audioResp.VoiceSentiment, // Audio servisinden geldi
			Speaker:         displaySpeaker,           // DB'den çözüldü
			SimilarityScore: audioResp.SimilarityScore,
		}
		database.DB.Create(&newSegment)

		// Frontend'e yanıt gönder
		response := map[string]interface{}{
			"type": "live_analysis",
			"payload": models.LiveAnalysisResult{
				Start:           finalStart,
				End:             finalEnd,
				Text:            seg.Text,
				TextSentiment:   textSentiment,
				VoiceSentiment:  audioResp.VoiceSentiment,
				Speaker:         displaySpeaker,
				SimilarityScore: audioResp.SimilarityScore,
			},
		}

		mu.Lock()
		if err := conn.WriteJSON(response); err != nil {
			log.Println("WS Write Error:", err)
		}
		mu.Unlock()
	}
}
