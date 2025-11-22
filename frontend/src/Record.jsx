// src/Record.jsx
import React, { useState, useRef, useEffect } from 'react';
import './Record.css';

const WS_URL = 'ws://localhost:8080/ws';

const Record = () => {
  const [isRecording, setIsRecording] = useState(false);
  const [duration, setDuration] = useState(0);
  const [wordCount, setWordCount] = useState(0);
  const [topic, setTopic] = useState('Tespit Ediliyor...');

  // Referanslar
  const socketRef = useRef(null);
  const audioContextRef = useRef(null); // AudioContext referansı
  const processorRef = useRef(null);    // İşlemci referansı
  const inputRef = useRef(null);        // Mikrofon girişi referansı
  const timerRef = useRef(null);

  const formatTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  // --- 1. Float32'yi Int16 PCM'e Çeviren Yardımcı Fonksiyon ---
  // Tarayıcı sesi -1.0 ile 1.0 arasında (float) verir.
  // VAD ise -32768 ile 32767 arasında (int16) ister.
  const floatTo16BitPCM = (input) => {
    const output = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
      const s = Math.max(-1, Math.min(1, input[i]));
      // 16-bit scale: 0x7FFF = 32767
      output[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
    }
    return output.buffer; // ArrayBuffer döner
  };

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

      socketRef.current = new WebSocket(WS_URL);

      socketRef.current.onopen = () => {
        console.log('WebSocket Bağlandı');
        initAudioProcessing(stream); // Yeni işlem fonksiyonunu çağır
      };

      socketRef.current.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.words) setWordCount(data.words);
          if (data.topic) setTopic(data.topic);
        } catch (e) {
          console.log("Mesaj parse edilemedi:", event.data);
        }
      };

      socketRef.current.onerror = (error) => console.error('WebSocket Hatası:', error);
      socketRef.current.onclose = () => console.log('WebSocket Kapandı');

    } catch (err) {
      console.error('Mikrofon erişim hatası:', err);
      alert("Mikrofon izni gerekli!");
    }
  };

  // --- 2. AudioContext ile PCM İşleme ---
  const initAudioProcessing = (stream) => {
    // VAD için en ideal Sample Rate: 16000 Hz
    const audioContext = new (window.AudioContext || window.webkitAudioContext)({
      sampleRate: 16000,
    });
    audioContextRef.current = audioContext;

    // Mikrofon kaynağını oluştur
    const source = audioContext.createMediaStreamSource(stream);
    inputRef.current = source;

    // İşlemci düğümü oluştur (BufferSize: 4096 sample)
    // 4096 sample / 16000 Hz ≈ 256ms veri paketleri
    // Buffer size 256, 512, 1024, 2048, 4096, 8192, 16384 olabilir.
    const bufferSize = 4096;
    const processor = audioContext.createScriptProcessor(bufferSize, 1, 1); // 1 Giriş (Mono), 1 Çıkış
    processorRef.current = processor;

    processor.onaudioprocess = (e) => {
      if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) return;

      // Sol kanalı al (Mono olduğu için yeterli)
      const inputData = e.inputBuffer.getChannelData(0);

      // Float32 verisini Int16 PCM verisine çevir
      const pcmData = floatTo16BitPCM(inputData);

      // Binary olarak gönder
      socketRef.current.send(pcmData);
    };

    // Bağlantıları kur: Source -> Processor -> Destination (Hoparlöre gitmesin diye destination'a bağlamasak da olur
    // ama bazen garbage collection durdurur, o yüzden dummy bir bağlantı genelde yapılır ama burada gerek yok)
    source.connect(processor);
    processor.connect(audioContext.destination);

    setIsRecording(true);
    startTimer();
  };

  const stopRecording = () => {
    // AudioContext ve Processor temizliği
    if (inputRef.current) inputRef.current.disconnect();
    if (processorRef.current) {
      processorRef.current.disconnect();
      processorRef.current.onaudioprocess = null;
    }
    if (audioContextRef.current) audioContextRef.current.close();

    // Işıkları söndür (Stream tracklerini durdur)
    if (inputRef.current && inputRef.current.mediaStream) {
        inputRef.current.mediaStream.getTracks().forEach(track => track.stop());
    }

    // Socket kapat
    if (socketRef.current) {
      socketRef.current.close();
    }

    // Timer durdur
    clearInterval(timerRef.current);
    setIsRecording(false);
  };

  const toggleRecording = () => {
    if (isRecording) {
      stopRecording();
    } else {
      setDuration(0);
      setWordCount(0);
      startRecording();
    }
  };

  const startTimer = () => {
    timerRef.current = setInterval(() => {
      setDuration((prev) => prev + 1);
    }, 1000);
  };

  useEffect(() => {
    return () => {
      stopRecording();
    };
  }, []);

  return (
    <div className="record-container">
      <h1 className="status-text">
        {isRecording ? 'Training...' : 'Tap to Record'}
      </h1>

      <div className="mic-wrapper">
        <button
          className={`mic-button ${isRecording ? 'recording' : ''}`}
          onClick={toggleRecording}
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="mic-icon"
          >
            <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"></path>
            <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
            <line x1="12" y1="19" x2="12" y2="23"></line>
            <line x1="8" y1="23" x2="16" y2="23"></line>
          </svg>
        </button>
      </div>

      <div className="stats-card">
        <div className="stat-item">
          <span className="stat-label">Duration</span>
          <span className="stat-value">{formatTime(duration)}</span>
        </div>
        <div className="stat-item">
          <span className="stat-label">Words</span>
          <span className="stat-value">{wordCount}</span>
        </div>
        <div className="stat-item">
          <span className="stat-label">Topic</span>
          <span className="stat-value">{topic}</span>
        </div>
      </div>

      <div className="details-list">
        <div className="detail-row">
          <span className="detail-label">Duration</span>
          <span className="detail-value">01:23</span>
        </div>
        <div className="detail-row">
          <span className="detail-label">Topic</span>
          <span className="detail-value">Technology</span>
        </div>
      </div>
    </div>
  );
};

export default Record;