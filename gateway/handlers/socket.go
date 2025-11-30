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

	database.DB.Exec("INSERT INTO records(id, date) values(?, ?)", sessionID, time.Now().Format("2006-01-02 15:04:05"))

	vad, _ := webrtcvad.New()
	vad.SetMode(3)

	var (
		audioBuffer    []byte
		currentSegment []byte
		isSpeaking     bool
		silenceCounter int
		bytesProcessed int
	)

	connLock := &sync.Mutex{}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı kesildi:", sessionID)
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

				// Go Routine içinde servislere gönder
				go processAndRespond(sessionID, segmentCopy, offsetSec, conn, connLock)

				currentSegment = nil
				isSpeaking = false
				silenceCounter = 0
			}
		}
	}
}

// Yardımcı fonksiyon: Servisleri çağırır ve sonucu WS'den döner
func processAndRespond(recordID string, pcmData []byte, offset float64, conn *websocket.Conn, mu *sync.Mutex) {
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

		finalStart := offset + seg.Start
		finalEnd := offset + seg.End

		database.DB.Exec(`INSERT INTO segments(record_id, start_offset, end_offset, text, text_sentiment, voice_sentiment, speaker) 
				 values(?, ?, ?, ?, ?, ?, ?)`,
			recordID, finalStart, finalEnd, analyzeResp.Text, analyzeResp.TextSentiment, analyzeResp.VoiceSentiment, analyzeResp.Speaker)

		response := map[string]interface{}{
			"type": "live_analysis",
			"payload": models.LiveAnalysisResult{
				Start:          finalStart,
				End:            finalEnd,
				Text:           analyzeResp.Text,
				TextSentiment:  analyzeResp.TextSentiment,
				VoiceSentiment: analyzeResp.VoiceSentiment,
				Speaker:        analyzeResp.Speaker,
			},
		}

		mu.Lock()
		conn.WriteJSON(response)
		mu.Unlock()
	}
}
