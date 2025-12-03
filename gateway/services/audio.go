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
	AudioServiceURL   = "http://localhost:5001/" // Audio Service (Ses İşleme)
	TextServiceURL    = "http://localhost:5002/" // Text Service (Metin İşleme)
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// CallWhisperService: Sesi metne çevirmek için (Whisper)
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

// CallAudioAnalyzeService: Sadece ses analizi (Voice Sentiment + Speaker Identification)
func CallAudioAnalyzeService(payload models.ServicePayload) (models.ServicePayload, error) {
	// Python servisi "wav_file" alanını bekliyor
	requestBody := map[string]interface{}{
		"wav_file": payload.WavFile,
	}
	jsonData, _ := json.Marshal(requestBody)

	// Endpoint: /analyze_audio
	resp, err := httpClient.Post(AudioServiceURL+"analyze_audio", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return models.ServicePayload{}, err
	}
	defer resp.Body.Close()

	// Gelen yanıtı parse et
	var responseMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseMap); err != nil {
		return models.ServicePayload{}, err
	}

	// Güvenli tip dönüşümü (Type Assertion)
	voiceSent, _ := responseMap["voice_sentiment"].(string)
	speaker, _ := responseMap["speaker"].(string)
	simScore, _ := responseMap["similarity_score"].(float64)

	result := models.ServicePayload{
		VoiceSentiment:  voiceSent,
		Speaker:         speaker,
		SimilarityScore: simScore,
	}
	return result, nil
}

// CallTextSentimentService: Sadece metin duygu analizi
func CallTextSentimentService(text string) (string, error) {
	requestBody := map[string]string{"text": text}
	jsonData, _ := json.Marshal(requestBody)

	// Endpoint: /sentiment
	resp, err := httpClient.Post(TextServiceURL+"sentiment", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "Hata", err
	}
	defer resp.Body.Close()

	var responseMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseMap); err != nil {
		return "Hata", err
	}

	sentiment, _ := responseMap["sentiment"].(string)
	return sentiment, nil
}

// CallIdentificateService: Kullanıcı ses kaydı (Speaker Enrollment)
func CallIdentificateService(userID uint, wavData []byte) error {
	payload := models.ServicePayload{
		Speaker: fmt.Sprintf("%d", userID),
		WavFile: wavData,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Audio Service üzerindeki identificate endpoint'i
	endpoint := AudioServiceURL + "identificate"

	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Audio servisi hata döndü (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Yardımcı Fonksiyonlar (WebM -> WAV, WAV Header)

func ConvertWebMToWav(webmData []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-ar", "16000", "-ac", "1", "-f", "wav", "pipe:1")
	cmd.Stdin = bytes.NewReader(webmData)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg dönüşüm hatası: %v", err)
	}
	return out.Bytes(), nil
}

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
