import os
import io
import json
import base64
import logging
import numpy as np
import pandas as pd
import librosa
import tensorflow as tf
from flask import Flask, request, jsonify
from joblib import load
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.base import BaseEstimator, TransformerMixin

# Loglama ayarları
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = Flask(__name__)

# Dizin Ayarları
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
SPEAKER_DB_FILE = os.path.join(BASE_DIR, 'speakers_db.json')
SENTIMENT_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'sentiment')
RECOGNITION_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'recognition')

# --- FeatureSelector Sınıfı (Pickle yüklemesi için gerekli) ---
class FeatureSelector(BaseEstimator, TransformerMixin):
    def __init__(self, feature_indices):
        self.feature_indices = feature_indices

    def fit(self, X, y=None):
        return self

    def transform(self, X, y=None):
        if hasattr(X, 'iloc'):
            return X.iloc[:, self.feature_indices]
        else:
            return X[:, self.feature_indices]

# --- Ses Modellerini Yöneten Sınıf ---
class AudioModels:
    def __init__(self):
        logging.info("Audio Service başlatılıyor...")

        # GPU ayarı (Opsiyonel)
        try:
            tf.config.set_visible_devices([], 'GPU')
        except:
            pass

        self.sentiment_ready = False
        self.recognition_ready = False

        self.load_sentiment_models()
        self.load_recognition_models()

        self.speaker_vectors = {}
        self.load_speaker_db()

    def load_sentiment_models(self):
        try:
            logging.info(f"Sentiment modelleri yükleniyor...: {SENTIMENT_MODEL_DIR}")
            self.sent_scaler = load(os.path.join(SENTIMENT_MODEL_DIR, 'scaler.pkl'))
            self.sent_selector = load(os.path.join(SENTIMENT_MODEL_DIR, 'selector.pkl'))
            self.sent_label_encoder = load(os.path.join(SENTIMENT_MODEL_DIR, 'label_encoder.pkl'))
            self.sent_model = tf.keras.models.load_model(os.path.join(SENTIMENT_MODEL_DIR, 'best_model.keras'))
            self.sentiment_ready = True
        except Exception as e:
            logging.error(f"Sentiment model hatası: {e}")

    def load_recognition_models(self):
        try:
            logging.info(f"Recognition modelleri yükleniyor...: {RECOGNITION_MODEL_DIR}")
            self.rec_scaler = load(os.path.join(RECOGNITION_MODEL_DIR, 'scaler.pkl'))
            self.rec_selector = load(os.path.join(RECOGNITION_MODEL_DIR, 'selector.pkl'))
            self.recognition_ready = True
        except Exception as e:
            logging.error(f"Recognition model hatası: {e}")

    def load_speaker_db(self):
        if os.path.exists(SPEAKER_DB_FILE):
            try:
                with open(SPEAKER_DB_FILE, 'r') as f:
                    self.speaker_vectors = json.load(f)
            except:
                self.speaker_vectors = {}
        else:
            self.speaker_vectors = {}

    def save_speaker_db(self):
        try:
            os.makedirs(os.path.dirname(SPEAKER_DB_FILE), exist_ok=True)
            with open(SPEAKER_DB_FILE, 'w') as f:
                json.dump(self.speaker_vectors, f)
        except Exception as e:
            logging.error(f"DB kayıt hatası: {e}")

    def add_speaker_vector(self, user_id, vector):
        if user_id not in self.speaker_vectors:
            self.speaker_vectors[user_id] = []
        if isinstance(vector, np.ndarray):
            vector = vector.tolist()
        self.speaker_vectors[user_id].append(vector)
        self.save_speaker_db()

models = AudioModels()

# --- Helper Fonksiyonlar (Feature Extraction vb.) ---
def extract_features_from_bytes(wav_bytes, sr=None):
    try:
        with io.BytesIO(wav_bytes) as wav_buffer:
            audio, sample_rate = librosa.load(wav_buffer, sr=sr)

        # ... (Önceki kodun aynısı: zero_crossing, mfcc, vb. hesaplama) ...
        # Kodun okunabilirliği için detayları kısaltıyorum, app.py'deki
        # extract_features_from_bytes içeriğinin AYNISINI buraya yapıştırın.

        # Kısaca örnek:
        mfcc = librosa.feature.mfcc(y=audio, sr=sample_rate, n_mfcc=40)
        mfcc_mean = np.mean(mfcc, axis=1)
        mfcc_std = np.std(mfcc, axis=1)
        # ... Diğer özellikler ...

        # Placeholder (Feature extract kodunun tamamını buraya almalısınız)
        # Şimdilik app.py'deki aynı fonksiyonun çalıştığını varsayıyoruz.
        # Bu kısım app.py'deki ile BİREBİR aynı kalmalı.
        return _extract_features_impl(audio, sample_rate)

    except Exception as e:
        logging.error(f"Feature extraction error: {e}")
        raise e

def _extract_features_impl(audio, sample_rate):
    # Bu fonksiyon app.py'deki feature çıkarma lojiğinin aynısıdır.
    # Yer kaplamaması için özetlenmiştir. Lütfen app.py'den kopyalayın.
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

    features = np.hstack([
        zero_crossing, spectral_centroid, spectral_rolloff, spectral_bandwidth,
        contrast_mean, contrast_std, chroma_stft_mean, chroma_stft_std,
        rms_mean, melspectrogram_mean, melspectrogram_std, flatness_mean,
        poly_mean, mfcc_mean, mfcc_std, energy
    ])
    return features

def get_column_names():
    # app.py'deki get_column_names fonksiyonunun aynısı
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

# --- İşlem Mantığı ---

def process_voice_sentiment(raw_features):
    if not models.sentiment_ready: return "ModelNotLoaded"
    try:
        columns = get_column_names()
        df = pd.DataFrame([raw_features], columns=columns)
        X_selected = df[models.sent_selector].values
        X_scaled = models.sent_scaler.transform(X_selected)
        X_reshaped = X_scaled.reshape(X_scaled.shape[0], X_scaled.shape[1], 1)
        preds = models.sent_model.predict(X_reshaped, verbose=0)
        pred_class = np.argmax(preds, axis=1)
        return models.sent_label_encoder.inverse_transform(pred_class)[0]
    except Exception as e:
        logging.error(f"Voice sentiment error: {e}")
        return "Error"

def process_recognition_vector(raw_features):
    if not models.recognition_ready: return raw_features
    try:
        X = raw_features.reshape(1, -1)
        X_scaled = models.rec_scaler.transform(X)
        return models.rec_selector.transform(X_scaled)[0]
    except:
        return raw_features

def identify_speaker_logic(processed_vector):
    if not models.speaker_vectors: return "Unknown", 0.0
    best_user = "Unknown"
    best_score = -1.0
    input_vec = processed_vector.reshape(1, -1)

    for user_id, vectors_list in models.speaker_vectors.items():
        db_vectors = np.array(vectors_list)
        if db_vectors.shape[1] != input_vec.shape[1]: continue
        similarities = cosine_similarity(input_vec, db_vectors)
        max_sim = np.max(similarities)
        if max_sim > best_score:
            best_score = max_sim
            best_user = user_id
    return best_user, float(best_score)

# --- Endpoints ---

@app.route('/identificate', methods=['POST'])
def identificate_user():
    try:
        data = request.json
        user_id = data['speaker']
        wav_bytes = base64.b64decode(data['wav_file'])

        raw_features = extract_features_from_bytes(wav_bytes)
        processed_vector = process_recognition_vector(raw_features)

        models.add_speaker_vector(user_id, processed_vector)
        return jsonify({"status": "success", "message": f"User {user_id} saved."})
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/analyze_audio', methods=['POST'])
def analyze_audio():
    """
    Sadece SES analizi yapar: Voice Sentiment + Speaker Identification
    """
    try:
        data = request.json
        wav_bytes = base64.b64decode(data['wav_file'])

        # 1. Özellik Çıkar
        raw_features = extract_features_from_bytes(wav_bytes)

        # 2. Ses Duygusu
        voice_sentiment = process_voice_sentiment(raw_features)

        # 3. Konuşmacı Tanıma
        rec_vector = process_recognition_vector(raw_features)
        speaker_id, speaker_score = identify_speaker_logic(rec_vector)

        return jsonify({
            "voice_sentiment": voice_sentiment,
            "speaker": speaker_id,
            "similarity_score": speaker_score,
            "status": "success"
        })
    except Exception as e:
        logging.error(f"Audio analyze error: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    # Bu servis 5001 portunda çalışmaya devam eder
    app.run(host='0.0.0.0', port=5001, debug=False)