package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
)

const (
	SampleRate       = 16000
	BitDepth         = 16
	Channels         = 1
	PacketSize       = 640            // 20ms frame
	MinSegmentBytes  = SampleRate * 2 // En az 1 saniyelik veri birikmeden gönderme (Memory Optimization)
	TargetServiceURL = "http://localhost:5000/"
	BytesPerSecond   = SampleRate * (BitDepth / 8) * Channels
)

// WhisperSegment WhisperX servisinden dönen segment yapısı
type WhisperSegment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type WhisperResult struct {
	Segments []WhisperSegment `json:"segments"`
	Language string           `json:"language"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HTTP Client global tanımlanarak tekrar tekrar oluşturulması engellenir (Connection Pooling)
var httpClient = &http.Client{Timeout: 60 * time.Second}

// WAV Başlığı oluşturucu (Harici kütüphane bağımlılığını azaltmak için manuel yazım)
func writeWavHeader(w io.Writer, dataLength int) error {
	// Dosya boyutu + 36 byte header
	fileSize := dataLength + 36

	// Header tamponu
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, int32(fileSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, int32(16)) // Chunk size
	binary.Write(buf, binary.LittleEndian, int16(1))  // Format (1 = PCM)
	binary.Write(buf, binary.LittleEndian, int16(Channels))
	binary.Write(buf, binary.LittleEndian, int32(SampleRate))
	binary.Write(buf, binary.LittleEndian, int32(SampleRate*Channels*BitDepth/8)) // ByteRate
	binary.Write(buf, binary.LittleEndian, int16(Channels*BitDepth/8))            // BlockAlign
	binary.Write(buf, binary.LittleEndian, int16(BitDepth))

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, int32(dataLength))

	_, err := w.Write(buf.Bytes())
	return err
}

// Sesi Whisper'a gönderir ve dönen JSON'a göre dosyayı dilimleyip kaydeder
func processAndSaveAudio(sessionID string, pcmData []byte, chunkIndex int) {
	// 1. WhisperX Servisine İstek At
	req, err := http.NewRequest("POST", TargetServiceURL, bytes.NewReader(pcmData))
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
		log.Printf("[%s] Sunucu hatası: %s", sessionID, resp.Status)
		return
	}

	// 2. JSON Yanıtını Parse Et
	var result WhisperResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[%s] JSON decode hatası: %v", sessionID, err)
		return
	}

	// 3. Dizin Kontrolü (sessions/session_ID)
	sessionDir := filepath.Join("sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		log.Printf("[%s] Klasör oluşturma hatası: %v", sessionID, err)
		return
	}

	// 4. Segmentasyon ve WAV Kaydı
	// Whisper'dan gelen her bir segment için orijinal PCM verisini kesiyoruz.
	for i, segment := range result.Segments {
		// Saniye -> Byte Offset Çevirimi
		startByte := int(segment.Start * float64(BytesPerSecond))
		endByte := int(segment.End * float64(BytesPerSecond))

		// Sınır Kontrolleri (Index Out of Range hatasını önlemek için)
		if startByte < 0 {
			startByte = 0
		}
		if endByte > len(pcmData) {
			endByte = len(pcmData)
		}
		if startByte >= endByte {
			continue
		}

		// Dilimleme (Memory Efficient: Yeni allocation yapmaz, sadece referans alır)
		segmentPCM := pcmData[startByte:endByte]

		// Dosya Adı: segment_CHUNKID_SEGMENTID.wav
		filename := fmt.Sprintf("segment_%d_%d.wav", chunkIndex, i)
		filePath := filepath.Join(sessionDir, filename)

		f, err := os.Create(filePath)
		if err != nil {
			log.Printf("Dosya oluşturma hatası: %v", err)
			continue
		}

		if err := writeWavHeader(f, len(segmentPCM)); err != nil {
			log.Printf("Header yazma hatası: %v", err)
			f.Close()
			continue
		}

		if _, err := f.Write(segmentPCM); err != nil {
			log.Printf("Veri yazma hatası: %v", err)
		}

		f.Close()
		log.Printf("[%s] Kaydedildi: %s (Text: %s)", sessionID, filename, segment.Text)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	log.Printf("Yeni oturum: %s", sessionID)

	// VAD Yapılandırması
	v, err := webrtcvad.New()
	if err != nil {
		log.Println(err)
		return
	}
	// Mode 3: En agresif gürültü filtresi (konuşma tespiti için daha hassas)
	if err := v.SetMode(3); err != nil {
		log.Println(err)
		return
	}

	var (
		currentSegment []byte
		notSpeechCount = 0
		audioBuffer    = make([]byte, 0)
		chunkCounter   = 0
	)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Bağlantı koptu:", sessionID)
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
				// Konuşma yoksa da bir miktar sessizliği buffer'a ekle ki kesilmeler sert olmasın
				if len(currentSegment) > 0 {
					currentSegment = append(currentSegment, frame...)
				}
			}

			// 500ms (25 frame) sessizlik varsa ve buffer doluysa gönder
			if notSpeechCount > 25 && len(currentSegment) > MinSegmentBytes {
				segmentToSend := make([]byte, len(currentSegment))
				copy(segmentToSend, currentSegment)

				chunkID := chunkCounter
				chunkCounter++

				go processAndSaveAudio(sessionID, segmentToSend, chunkID)

				currentSegment = make([]byte, 0)
				notSpeechCount = 0
			}
		}
	}

	// Bağlantı kapandığında kalan ses verilerini gönder
	log.Printf("[%s] Bağlantı kapalı, kalan buffer'lar gönderiliyor...", sessionID)

	// 1. audioBuffer'da kalan veriler (PacketSize'dan küçük frame'ler)
	for len(audioBuffer) >= PacketSize {
		frame := audioBuffer[:PacketSize]
		audioBuffer = audioBuffer[PacketSize:]

		isSpeaking, err := v.Process(SampleRate, frame)
		if err != nil {
			continue
		}

		if isSpeaking {
			currentSegment = append(currentSegment, frame...)
		} else {
			if len(currentSegment) > 0 {
				currentSegment = append(currentSegment, frame...)
			}
		}
	}

	// 2. Kalan audioBuffer'da PacketSize'dan küçük veri varsa ve currentSegment'e ekle
	if len(audioBuffer) > 0 && len(currentSegment) > 0 {
		currentSegment = append(currentSegment, audioBuffer...)
	}

	// 3. currentSegment'te kalan tüm veriyi gönder
	if len(currentSegment) > 0 {
		segmentToSend := make([]byte, len(currentSegment))
		copy(segmentToSend, currentSegment)

		chunkID := chunkCounter
		go processAndSaveAudio(sessionID, segmentToSend, chunkID)

		log.Printf("[%s] Son segment gönderildi (Boyut: %d bytes, ~%.2f sn)", sessionID, len(currentSegment), float64(len(currentSegment))/float64(BytesPerSecond))
	}
}

func main() {
	// Sessions klasörünü ana dizinde oluştur
	if err := os.Mkdir("sessions", 0755); err != nil && !os.IsExist(err) {
		log.Fatal("Sessions klasörü oluşturulamadı:", err)
	}

	http.HandleFunc("/ws", wsHandler)
	log.Println("WebSocket sunucusu 8080 portunda başlatılıyor...")
	log.Println("WAV dosyaları 'sessions/' altına kaydedilecek.")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
