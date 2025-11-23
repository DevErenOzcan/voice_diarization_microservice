import os
import io
import base64
import logging
import numpy as np
import pandas as pd
import librosa
import tensorflow as tf
from flask import Flask, request, jsonify
from joblib import load

# Loglama ayarları
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = Flask(__name__)

# --- Modelleri Yükleme ---
# Global değişkenler olarak tanımlıyoruz, uygulama başlarken hafızaya alınacaklar.
class MLModels:
    def __init__(self):
        logging.info("Modeller yükleniyor...")
        try:
            # GPU kullanımını kapat (CPU tabanlı inference genellikle production için daha stabil olabilir)
            tf.config.set_visible_devices([], 'GPU')

            # Resolve models directory: support env override for flexibility in different runtimes
            base_dir = os.path.dirname(os.path.abspath(__file__))
            env_model_dir = os.getenv('ANALYZE_MODEL_DIR')
            if env_model_dir:
                # If env var is relative, treat it relative to this file
                if not os.path.isabs(env_model_dir):
                    model_dir = os.path.join(base_dir, env_model_dir)
                else:
                    model_dir = env_model_dir
                logging.info(f"ANALYZE_MODEL_DIR provided, using: {model_dir}")
            else:
                model_dir = os.path.join(base_dir, 'models')
                logging.info(f"ANALYZE_MODEL_DIR not set, defaulting to: {model_dir}")

            scaler_path = os.path.join(model_dir, 'scaler.pkl')
            selector_path = os.path.join(model_dir, 'selector.pkl')
            label_encoder_path = os.path.join(model_dir, 'label_encoder.pkl')
            best_model_path = os.path.join(model_dir, 'best_model.keras')

            logging.info(f"Model dizini: {model_dir}")

            # Check files exist and raise a clear error if not
            missing = [p for p in (scaler_path, selector_path, label_encoder_path, best_model_path) if not os.path.exists(p)]
            if missing:
                logging.error("Aşağıdaki model dosyaları bulunamadı:")
                for m in missing:
                    logging.error(f"  - {m}")
                raise FileNotFoundError(f"Eksik model dosyaları: {missing}")

            self.scaler = load(scaler_path)
            self.selector = load(selector_path)
            self.label_encoder = load(label_encoder_path)
            self.best_model = tf.keras.models.load_model(best_model_path)
            logging.info("Tüm modeller başarıyla yüklendi.")
        except Exception as e:
            logging.error(f"Model yükleme hatası: {e}")
            raise e

# Modelleri başlat
models = MLModels()

# --- Yardımcı Fonksiyonlar ---

def get_column_names():
    """
    consumers.py dosyasındaki sütun isimleri mantığının birebir aynısı.
    Selector'un doğru çalışması için bu sıralama hayati önem taşır.
    """
    columns = (
            ['zero_crossing', 'centroid_mean', 'rolloff_mean', 'bandwidth_mean'] +
            [f'contrast_mean_{i}' for i in range(7)] +
            [f'contrast_std_{i}' for i in range(7)] +
            [f'chroma_stft_mean_{i}' for i in range(12)] +
            [f'chroma_stft_std_{i}' for i in range(12)] +
            ['rms_mean', 'melspectrogram_mean', 'melspectrogram_std', 'flatness_mean'] +
            [f'poly_mean_{i}' for i in range(2)] +
            [f'mfcc_mean_{i}' for i in range(40)] +
            [f'mfcc_std_{i}' for i in range(40)] +
            ['energy']
    )
    return columns

def extract_features_from_bytes(wav_bytes, sr=16000):
    """
    Librosa kullanarak byte verisinden öznitelik çıkarır.
    Eski projedeki 'extract_features' fonksiyonunun aynısıdır.
    """
    try:
        # Byte verisini sanal dosya olarak oku
        with io.BytesIO(wav_bytes) as wav_buffer:
            # Go tarafı 16000 sample rate ile gönderiyor
            audio, sample_rate = librosa.load(wav_buffer, sr=sr)

        # 1. Zero Crossing Rate
        zero_crossing = np.mean(librosa.feature.zero_crossing_rate(y=audio).T, axis=0)

        # 2. Spectral Features
        spectral_centroid = np.mean(librosa.feature.spectral_centroid(y=audio, sr=sample_rate).T, axis=0)
        spectral_rolloff = np.mean(librosa.feature.spectral_rolloff(y=audio, sr=sample_rate).T, axis=0)
        spectral_bandwidth = np.mean(librosa.feature.spectral_bandwidth(y=audio, sr=sample_rate).T, axis=0)

        spectral_contrast = librosa.feature.spectral_contrast(y=audio, sr=sample_rate)
        contrast_mean = np.mean(spectral_contrast, axis=1)
        contrast_std = np.std(spectral_contrast, axis=1)

        # 3. Chroma
        chroma_stft = librosa.feature.chroma_stft(y=audio, sr=sample_rate)
        chroma_stft_mean = np.mean(chroma_stft, axis=1)
        chroma_stft_std = np.std(chroma_stft, axis=1)

        # 4. RMS
        rms_mean = np.mean(librosa.feature.rms(y=audio))

        # 5. Mel Spectrogram
        mel_spectrogram = librosa.feature.melspectrogram(y=audio, sr=sample_rate)
        melspectrogram_mean = np.mean(mel_spectrogram)
        melspectrogram_std = np.std(mel_spectrogram)

        # 6. Flatness
        flatness_mean = np.mean(librosa.feature.spectral_flatness(y=audio))

        # 7. Poly Features
        poly_features = librosa.feature.poly_features(y=audio, sr=sample_rate, order=1)
        poly_mean = np.mean(poly_features, axis=1)

        # 8. MFCC
        mfcc = librosa.feature.mfcc(y=audio, sr=sample_rate, n_mfcc=40)
        mfcc_mean = np.mean(mfcc, axis=1)
        mfcc_std = np.std(mfcc, axis=1)

        # 9. Energy
        energy = np.sum(audio ** 2)

        # Vektör birleştirme (Sıralama çok önemli)
        features = np.hstack([
            zero_crossing, spectral_centroid, spectral_rolloff, spectral_bandwidth,
            contrast_mean, contrast_std, chroma_stft_mean, chroma_stft_std,
            rms_mean, melspectrogram_mean, melspectrogram_std, flatness_mean,
            poly_mean, mfcc_mean, mfcc_std, energy
        ])

        return features

    except Exception as e:
        logging.error(f"Öznitelik çıkarma hatası: {e}")
        raise e

def predict_sentiment(df_features):
    """
    Oluşturulan DataFrame üzerinden model tahminlemesi yapar.
    """
    try:
        # 1. Selector ile özellikleri seç
        new_features = df_features[models.selector].values

        # 2. Scaler ile ölçeklendir
        new_features_scaled = models.scaler.transform(new_features)

        # 3. CNN için Reshape (samples, features, 1)
        new_features_scaled = new_features_scaled.reshape(new_features_scaled.shape[0], new_features_scaled.shape[1], 1)

        # 4. Tahmin
        predictions = models.best_model.predict(new_features_scaled, verbose=0)

        # 5. Label Decoding
        predicted_classes = np.argmax(predictions, axis=1)
        predicted_labels = models.label_encoder.inverse_transform(predicted_classes)

        return predicted_labels[0] # Tek bir sonuç dönüyoruz

    except Exception as e:
        logging.error(f"Tahminleme hatası: {e}")
        return "Error"


@app.route('/', methods=['POST'])
def analyze():
    try:
        data = request.json
        if not data:
            return jsonify({"error": "Veri bulunamadı"}), 400

        # Go'daki struct yapısı:
        # SegmentID, WavFile ([]byte -> base64 string), Text, Language vb.
        segment_id = data.get('segment_id')
        wav_base64 = data.get('wav_file')
        text = data.get('text', '')
        language = data.get('language', '')

        if not wav_base64:
            return jsonify({"error": "wav_file eksik"}), 400

        # 1. Base64 Decode (Go []byte json olarak base64 string gönderir)
        wav_bytes = base64.b64decode(wav_base64)

        # 2. Feature Extraction
        features = extract_features_from_bytes(wav_bytes, sr=16000)

        # 3. DataFrame Oluşturma (Eski kodla uyumlu sütun isimleri)
        columns = get_column_names()

        # Sütun sayısı kontrolü (Opsiyonel güvenlik)
        if len(features) != len(columns):
            logging.warning(f"Feature sayısı uyuşmazlığı! Beklenen: {len(columns)}, Çıkan: {len(features)}")
            # Burada hata döndürmek yerine devam ediyoruz ama logluyoruz.

        df = pd.DataFrame([features], columns=columns)

        # 4. Tahminleme
        audio_sentiment = predict_sentiment(df)

        logging.info(f"Segment: {segment_id} | Tahmin: {audio_sentiment} | Text: {text}")

        # 5. Yanıt
        response = {
            "segment_id": segment_id,
            "text": text,
            "audio_sentiment": audio_sentiment,
            "language": language,
            "status": "success"
        }

        return jsonify(response)

    except Exception as e:
        logging.error(f"API Hatası: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    # Go main.go dosyasındaki 'AnalyzeServiceURL' http://localhost:5001/ olduğu için:
    app.run(host='0.0.0.0', port=5001, debug=True)
