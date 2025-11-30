package services

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"gateway/models"
)

const (
	WhisperServiceURL = "http://localhost:5000/"
	AnalyzeServiceURL = "http://localhost:5001/"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

func CallWhisperService(pcmData []byte) (models.ServicePayload, error) {
	resp, err := httpClient.Post(WhisperServiceURL, "application/octet-stream", bytes.NewReader(pcmData))
	if err != nil {
		return models.ServicePayload{}, err
	}
	defer resp.Body.Close()

	var result models.ServicePayload
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func CallAnalyzeService(payload models.ServicePayload) (models.ServicePayload, error) {
	jsonData, _ := json.Marshal(payload)
	resp, err := httpClient.Post(AnalyzeServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return models.ServicePayload{}, err
	}
	defer resp.Body.Close()

	var result models.ServicePayload
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func ConvertWebMToWav(webmData []byte) ([]byte, error) {
	// FFmpeg komutu:
	// -i pipe:0      -> Girdiyi standart inputtan (stdin) oku
	// -ar 16000      -> 16000 Hz örnekleme hızı (Speech modelleri için standart)
	// -ac 1          -> Tek kanal (Mono)
	// -f wav         -> Çıktı formatı WAV
	// pipe:1         -> Çıktıyı standart outputa (stdout) ver
	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-ar", "16000", "-ac", "1", "-f", "wav", "pipe:1")

	// Girdi (WebM) için pipe
	cmd.Stdin = bytes.NewReader(webmData)

	// Çıktı (WAV) için buffer
	var out bytes.Buffer
	cmd.Stdout = &out

	// Hata mesajlarını yakalamak isterseniz stderr'i de bağlayabilirsiniz (optional)
	// var stderr bytes.Buffer
	// cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg dönüşüm hatası: %v", err)
	}

	return out.Bytes(), nil
}

// CallIdentificateService: Kullanıcı ID'si ve ses dosyasını Analyze servisine gönderir
func CallIdentificateService(userID uint, wavData []byte) error {
	payload := models.ServicePayload{
		Speaker: fmt.Sprintf("%d", userID),
		WavFile: wavData,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// AnalyzeServiceURL -> "http://localhost:5001/" (audio.go başında tanımlı varsayılıyor)
	endpoint := AnalyzeServiceURL + "identificate"

	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Hata detayını okumaya çalışalım
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("analyze servisi hata döndü (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateWav fonksiyonu PCM datayı WAV formatına çevirir
func CreateWav(pcm []byte) []byte {
	buf := new(bytes.Buffer)
	writeWavHeader(buf, len(pcm))
	buf.Write(pcm)
	return buf.Bytes()
}

func writeWavHeader(w io.Writer, dataLength int) {
	binary.Write(w, binary.LittleEndian, []byte("RIFF"))
	binary.Write(w, binary.LittleEndian, int32(dataLength+36))
	binary.Write(w, binary.LittleEndian, []byte("WAVEfmt "))
	binary.Write(w, binary.LittleEndian, int32(16))
	binary.Write(w, binary.LittleEndian, int16(1))
	binary.Write(w, binary.LittleEndian, int16(1))
	binary.Write(w, binary.LittleEndian, int32(models.SampleRate))
	binary.Write(w, binary.LittleEndian, int32(models.SampleRate*2))
	binary.Write(w, binary.LittleEndian, int16(2))
	binary.Write(w, binary.LittleEndian, int16(16))
	binary.Write(w, binary.LittleEndian, []byte("data"))
	binary.Write(w, binary.LittleEndian, int32(dataLength))
}
