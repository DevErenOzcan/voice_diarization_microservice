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
from sklearn.base import BaseEstimator, TransformerMixin

# --- Yapılandırma ---
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
SENTIMENT_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'sentiment')
RECOGNITION_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'recognition')

app = Flask(__name__)

# --- Yardımcı Sınıflar ---
class FeatureSelector(BaseEstimator, TransformerMixin):
    """Pickle dosyasını yüklerken hata almamak için gerekli sınıf."""
    def __init__(self, feature_indices):
        self.feature_indices = feature_indices

    def fit(self, X, y=None):
        return self

    def transform(self, X, y=None):
        if hasattr(X, 'iloc'):
            return X.iloc[:, self.feature_indices]
        return X[:, self.feature_indices]

# --- Ana Model ve İşlem Sınıfı ---
class AudioService:
    def __init__(self):
        logging.info("Audio Service başlatılıyor...")
        self._configure_gpu()

        # Durum Bayrakları
        self.sentiment_ready = False
        self.recognition_ready = False

        # Modelleri Yükle
        self._load_sentiment_models()
        self._load_recognition_models()

    def _configure_gpu(self):
        try:
            tf.config.set_visible_devices([], 'GPU')
        except Exception:
            pass

    def _load_sentiment_models(self):
        try:
            logging.info(f"Sentiment modelleri yükleniyor: {SENTIMENT_MODEL_DIR}")
            self.sent_scaler = load(os.path.join(SENTIMENT_MODEL_DIR, 'scaler.pkl'))
            self.sent_selector = load(os.path.join(SENTIMENT_MODEL_DIR, 'selector.pkl'))
            self.sent_label_encoder = load(os.path.join(SENTIMENT_MODEL_DIR, 'label_encoder.pkl'))
            self.sent_model = tf.keras.models.load_model(os.path.join(SENTIMENT_MODEL_DIR, 'best_model.keras'))
            self.sentiment_ready = True
        except Exception as e:
            logging.error(f"Sentiment model hatası: {e}")

    def _load_recognition_models(self):
        try:
            logging.info(f"Recognition modelleri yükleniyor: {RECOGNITION_MODEL_DIR}")
            # TPOT Modeli ve ilgili dönüştürücüler
            self.rec_scaler = load(os.path.join(RECOGNITION_MODEL_DIR, 'scaler.pkl'))
            self.rec_selector = load(os.path.join(RECOGNITION_MODEL_DIR, 'selector.pkl'))
            self.rec_model = load(os.path.join(RECOGNITION_MODEL_DIR, 'tpot_best.pkl'))
            self.recognition_ready = True
        except Exception as e:
            logging.error(f"Recognition model hatası: {e}")

    # --- Feature Extraction (Birleştirilmiş) ---
    def extract_features(self, wav_bytes, sr=None):
        try:
            # 1. Byte verisini sese dönüştür
            with io.BytesIO(wav_bytes) as wav_buffer:
                audio, sample_rate = librosa.load(wav_buffer, sr=sr)

            # 2. Özellikleri hesapla
            zero_crossing = np.mean(librosa.feature.zero_crossing_rate(y=audio).T, axis=0)
            spectral_centroid = np.mean(librosa.feature.spectral_centroid(y=audio, sr=sample_rate).T, axis=0)
            spectral_rolloff = np.mean(librosa.feature.spectral_rolloff(y=audio, sr=sample_rate).T, axis=0)
            spectral_bandwidth = np.mean(librosa.feature.spectral_bandwidth(y=audio, sr=sample_rate).T, axis=0)

            spectral_contrast = librosa.feature.spectral_contrast(y=audio, sr=sample_rate)
            contrast_mean = np.mean(spectral_contrast, axis=1)
            contrast_std = np.std(spectral_contrast, axis=1)

            chroma_stft = librosa.feature.chroma_stft(y=audio, sr=sample_rate)
            chroma_stft_mean = np.mean(chroma_stft, axis=1)
            chroma_stft_std = np.std(chroma_stft, axis=1)

            rms_mean = np.mean(librosa.feature.rms(y=audio))

            mel_spectrogram = librosa.feature.melspectrogram(y=audio, sr=sample_rate)
            melspectrogram_mean = np.mean(mel_spectrogram)
            melspectrogram_std = np.std(mel_spectrogram)

            flatness_mean = np.mean(librosa.feature.spectral_flatness(y=audio))

            poly_features = librosa.feature.poly_features(y=audio, sr=sample_rate, order=1)
            poly_mean = np.mean(poly_features, axis=1)

            mfcc = librosa.feature.mfcc(y=audio, sr=sample_rate, n_mfcc=40)
            mfcc_mean = np.mean(mfcc, axis=1)
            mfcc_std = np.std(mfcc, axis=1)

            energy = np.sum(audio ** 2)

            # 3. Vektörü oluştur
            features = np.hstack([
                zero_crossing, spectral_centroid, spectral_rolloff, spectral_bandwidth,
                contrast_mean, contrast_std, chroma_stft_mean, chroma_stft_std,
                rms_mean, melspectrogram_mean, melspectrogram_std, flatness_mean,
                poly_mean, mfcc_mean, mfcc_std, energy
            ])
            return features

        except Exception as e:
            logging.error(f"Feature extraction error: {e}")
            raise e

    def get_column_names(self):
        return (
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

    # --- Tahmin Mantığı ---
    def predict_sentiment(self, raw_features):
        if not self.sentiment_ready: return "ModelNotLoaded"
        try:
            columns = self.get_column_names()
            df = pd.DataFrame([raw_features], columns=columns)

            # Seçim ve Ölçeklendirme
            X_selected = df[self.sent_selector].values
            X_scaled = self.sent_scaler.transform(X_selected)
            X_reshaped = X_scaled.reshape(X_scaled.shape[0], X_scaled.shape[1], 1)

            # Tahmin
            preds = self.sent_model.predict(X_reshaped, verbose=0)
            pred_class = np.argmax(preds, axis=1)
            return self.sent_label_encoder.inverse_transform(pred_class)[0]
        except Exception as e:
            logging.error(f"Voice sentiment error: {e}")
            return "Error"

    def identify_speaker(self, raw_features):
        if not self.recognition_ready: return "Unknown", 0.0

        try:
            # 1. Ham özellikleri boyutlandır (1, n_features)
            X = raw_features.reshape(1, -1)

            # 2. Scaler ve Selector işlemlerini uygula
            X_scaled = self.rec_scaler.transform(X)
            X_selected = self.rec_selector.transform(X_scaled)

            # 3. Model Tahmini
            if hasattr(self.rec_model, "predict_proba"):
                proba = self.rec_model.predict_proba(X_selected)[0]
                class_labels = self.rec_model.classes_

                # En yüksek olasılığa sahip sınıfı bul
                best_idx = np.argmax(proba)
                best_label = class_labels[best_idx]
                best_prob = float(proba[best_idx])

                return str(best_label), best_prob
            else:
                # Eger predict_proba desteklenmiyorsa düz tahmin al
                y_pred = self.rec_model.predict(X_selected)
                return str(y_pred[0]), 1.0

        except Exception as e:
            logging.error(f"Speaker prediction error: {e}")
            return "Error", 0.0

# Servis örneğini oluştur
audio_service = AudioService()

# --- Endpointler ---

@app.route('/analyze_audio', methods=['POST'])
def analyze_audio():
    try:
        data = request.json
        wav_b64 = data.get('wav_file')

        if not wav_b64:
            return jsonify({"error": "Missing wav_file"}), 400

        wav_bytes = base64.b64decode(wav_b64)

        # 1. Özellik Çıkar
        raw_features = audio_service.extract_features(wav_bytes)

        # 2. Duygu Analizi
        voice_sentiment = audio_service.predict_sentiment(raw_features)

        # 3. Konuşmacı Tanıma (TPOT Modeli İle)
        speaker_id, confidence_score = audio_service.identify_speaker(raw_features)

        return jsonify({
            "voice_sentiment": voice_sentiment,
            "speaker": speaker_id,
            "similarity_score": confidence_score,
            "status": "success"
        })
    except Exception as e:
        logging.error(f"Audio analyze error: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001, debug=False)