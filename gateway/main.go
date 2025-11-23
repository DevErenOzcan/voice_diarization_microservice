package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
)

const (
	SampleRate        = 16000
	BitDepth          = 16
	Channels          = 1
	PacketSize        = 640
	MinSegmentBytes   = SampleRate * 2 * 3
	WhisperServiceURL = "http://localhost:5000/"
	AnalyzeServiceURL = "http://localhost:5001/"
	BytesPerSecond    = SampleRate * (BitDepth / 8) * Channels
)

// API'den gelen ham yanıt yapısı
type WhisperAPIResponse struct {
	Segments []AnalyzePayload `json:"segments"`
	Language string           `json:"language"`
}

// İşlenmiş ve gönderilmeye hazır veri yapısı
type AnalyzePayload struct {
	SegmentID string  `json:"segment_id,omitempty"`
	WavFile   []byte  `json:"wav_file,omitempty"`
	Text      string  `json:"text"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Language  string  `json:"language"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

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

func sendToAnlyzeService(payload AnalyzePayload) {
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

	// Yanıt gövdesini oku
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Response body okuma hatası: %v", err)
		return
	}

	// Durum kodu 200 (OK) değilse hata olarak logla
	if resp.StatusCode != http.StatusOK {
		log.Printf("AnalyzeService hata döndürdü (%s): %s", resp.Status, string(body))
		return
	}

	// Başarılı yanıtı string olarak yazdır
	log.Printf("Analyze Servis Yanıtı: %s", string(body))
}

func sendToWhisperxService(sessionID string, pcmData []byte, chunkIndex int) {
	req, err := http.NewRequest("POST", WhisperServiceURL, bytes.NewReader(pcmData))
	if err != nil {
		log.Printf("[%s] Request oluşturma hatası: %v", sessionID, err)
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

	// Yanıtı yeni tanımladığımız struct'a decode ediyoruz
	var apiResult WhisperAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		log.Printf("[%s] JSON decode hatası: %v", sessionID, err)
		return
	}

	// Ayrıştırma ve işleme mantığı ayrı fonksiyona taşındı
	payloads := createAnalyzePayload(sessionID, pcmData, chunkIndex, apiResult)

	// Hazırlanan payload'ları asenkron servise gönder
	for _, payload := range payloads {
		go sendToAnlyzeService(payload)
	}
}

func createAnalyzePayload(sessionID string, pcmData []byte, chunkIndex int, result WhisperAPIResponse) []AnalyzePayload {
	var validSegments []AnalyzePayload

	for i, segment := range result.Segments {
		// Zaman damgasına göre byte hesaplamaları
		startByte := int(segment.Start * float64(BytesPerSecond))
		endByte := int(segment.End * float64(BytesPerSecond))

		// Sınır kontrolleri
		if startByte < 0 {
			startByte = 0
		}
		if endByte > len(pcmData) {
			endByte = len(pcmData)
		}
		if startByte >= endByte {
			continue
		}

		// WAV dosyası oluştur
		segmentPCM := pcmData[startByte:endByte]
		wavBuffer := new(bytes.Buffer)
		if err := writeWavHeader(wavBuffer, len(segmentPCM)); err == nil {
			wavBuffer.Write(segmentPCM)
		}

		// Segment ID'yi oluştur ve diğer bilgileri ekle
		segment.SegmentID = fmt.Sprintf("%s_%d_%d", sessionID, chunkIndex, i)
		segment.WavFile = wavBuffer.Bytes()
		segment.Language = result.Language // Ana wrapper'dan dili alıp segmente ekle
		validSegments = append(validSegments, segment)
	}

	return validSegments
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	log.Printf("Oturum başladı: %s", sessionID)

	v, err := webrtcvad.New()
	if err != nil {
		log.Println(err)
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
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		audioBuffer = append(audioBuffer, message...)

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

			// Sessizlik süresi dolduysa ve segment yeterince uzunsa gönder
			if notSpeechCount > 25 && len(currentSegment) > MinSegmentBytes {
				segmentToSend := make([]byte, len(currentSegment))
				copy(segmentToSend, currentSegment)

				go sendToWhisperxService(sessionID, segmentToSend, chunkCounter)

				chunkCounter++
				currentSegment = make([]byte, 0)
				notSpeechCount = 0
			}
		}
	}

	if len(currentSegment) > 0 {
		go sendToWhisperxService(sessionID, currentSegment, chunkCounter)
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Gateway başlatıldı (8080)...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
