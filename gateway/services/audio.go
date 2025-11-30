package services

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
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
