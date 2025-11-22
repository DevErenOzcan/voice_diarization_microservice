package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
)

const (
	SampleRate       = 16000
	PacketSize       = 640 // 320 samples * 2 bytes (16-bit) = 20ms frame for 16kHz
	MinSegmentBytes  = SampleRate * 2 * 3
	TargetServiceURL = "http://localhost:5000/"
)

type WhisperSegment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"` // Saniye cinsinden
	End   float64 `json:"end"`   // Saniye cinsinden
}

type WhisperResult struct {
	Segments []WhisperSegment `json:"segments"`
	Language string           `json:"language"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}
var httpClient = &http.Client{Timeout: 60 * time.Second}

func sendAudioToService(pcmData []byte) {
	req, err := http.NewRequest("POST", TargetServiceURL, bytes.NewReader(pcmData))
	if err != nil {
		log.Printf("Request oluşturma hatası: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	// Alternatif olarak: "audio/pcm" veya "audio/l16; rate=16000; channels=1"

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Gönderim hatası: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Sunucu hatası: %s", resp.Status)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	// Session ID oluştur
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	log.Printf("Yeni bağlantı: %s (Sesler servise iletilecek)", sessionID)

	// VAD Başlatma
	v, err := webrtcvad.New()
	if err != nil {
		log.Println(err)
		return
	}
	if err := v.SetMode(2); err != nil {
		log.Println(err)
		return
	}

	var currentSegment []byte
	notSpeechCount := 0
	previouslySpeaking := false
	audioBuffer := make([]byte, 0)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı koptu:", err)
			break
		}

		audioBuffer = append(audioBuffer, message...)

		for len(audioBuffer) >= PacketSize {
			frame := audioBuffer[:PacketSize]
			audioBuffer = audioBuffer[PacketSize:]

			isSpeaking, err := v.Process(SampleRate, frame)
			if err != nil {
				log.Println("VAD Hatası:", err)
				continue
			}

			if isSpeaking {
				if !previouslySpeaking {
					if len(currentSegment) > MinSegmentBytes {
						if notSpeechCount > 10 {
							notSpeechLen := notSpeechCount * PacketSize
							halfSilence := notSpeechLen / 2

							if len(currentSegment) >= halfSilence {
								splitIndex := len(currentSegment) - halfSilence
								segmentToSave := make([]byte, splitIndex)
								copy(segmentToSave, currentSegment[:splitIndex])
								go sendAudioToService(segmentToSave)

								newSegmentBuffer := make([]byte, halfSilence)
								copy(newSegmentBuffer, currentSegment[splitIndex:])
								currentSegment = newSegmentBuffer
							}
						}
					}
				}
				notSpeechCount = 0
			} else {
				notSpeechCount++
			}

			previouslySpeaking = isSpeaking
			currentSegment = append(currentSegment, frame...)
		}
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("WebSocket sunucusu 8080 portunda başlatılıyor...")
	log.Println("Ses verileri şu adrese yönlendirilecek:", TargetServiceURL)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
