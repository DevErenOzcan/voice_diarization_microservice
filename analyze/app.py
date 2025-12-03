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

from transformers import pipeline
from transformers import pipeline, AutoTokenizer, TFAutoModelForSequenceClassification

# Loglama ayarları
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = Flask(__name__)

# Dizin Ayarları
BASE_DIR = os.path.dirname(os.path.abspath(__file__))

SPEAKER_DB_FILE = os.path.join(BASE_DIR, 'speakers_db.json')

# Klasör yollarını projenize göre kontrol edin, varsayılan yapı:
SENTIMENT_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'sentiment')
RECOGNITION_MODEL_DIR = os.path.join(BASE_DIR, 'models', 'recognition')

# --- FeatureSelector Sınıfı ---
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

# --- Modelleri ve Veritabanını Yöneten Sınıf ---
class MLModels:
    def __init__(self):
        logging.info("Sistem başlatılıyor...")

        # GPU'yu devre dışı bırak (TensorFlow için opsiyonel)
        try:
            tf.config.set_visible_devices([], 'GPU')
        except:
            pass

        self.sentiment_ready = False
        self.recognition_ready = False
        self.text_sentiment_ready = False  # YENİ FLAG

        # 1. Sentiment Modellerini Yükle
        self.load_sentiment_models()

        # 2. Recognition Pre-processorlarını Yükle
        self.load_recognition_models()

        # 3. Speaker Veritabanını Yükle (JSON)
        self.speaker_vectors = {}
        self.load_speaker_db()

        # 4. Text Sentiment Modelini Yükle (YENİ)
        self.load_text_sentiment_model()

    def load_sentiment_models(self):
        """Duygu analizi için gerekli scaler, selector ve keras modelini yükler."""
        try:
            logging.info(f"Sentiment modelleri yükleniyor...: {SENTIMENT_MODEL_DIR}")
            self.sent_scaler = load(os.path.join(SENTIMENT_MODEL_DIR, 'scaler.pkl'))
            self.sent_selector = load(os.path.join(SENTIMENT_MODEL_DIR, 'selector.pkl'))
            self.sent_label_encoder = load(os.path.join(SENTIMENT_MODEL_DIR, 'label_encoder.pkl'))
            self.sent_model = tf.keras.models.load_model(os.path.join(SENTIMENT_MODEL_DIR, 'best_model.keras'))

            self.sentiment_ready = True
            logging.info("Sentiment modelleri BAŞARIYLA yüklendi.")
        except Exception as e:
            logging.error(f"Sentiment modelleri yüklenirken hata (Opsiyonel): {e}")
            self.sentiment_ready = False

    def load_recognition_models(self):
        """Recognition (Hoparlör Tanıma) için Scaler ve Selector yükler."""
        try:
            logging.info(f"Recognition modelleri yükleniyor...: {RECOGNITION_MODEL_DIR}")
            self.rec_scaler = load(os.path.join(RECOGNITION_MODEL_DIR, 'scaler.pkl'))
            self.rec_selector = load(os.path.join(RECOGNITION_MODEL_DIR, 'selector.pkl'))

            self.recognition_ready = True
            logging.info("Recognition pre-processorları BAŞARIYLA yüklendi.")
        except Exception as e:
            logging.error(f"Recognition modelleri yüklenirken hata oluştu: {e}")
            self.recognition_ready = False

    def load_text_sentiment_model(self):
        """Yerel klasörden Türkçe Metin Duygu Analizi modelini yükler."""
        try:
            model_path = os.path.join(BASE_DIR, 'models', 'text_sentiment')
            logging.info(f"Text Sentiment modeli yükleniyor: {model_path}")

            # Config dosyasını düzelttiğimiz için artık standart yükleme yapabiliriz
            tokenizer = AutoTokenizer.from_pretrained(model_path)

            # TensorFlow modelini yükle
            model = TFAutoModelForSequenceClassification.from_pretrained(model_path)

            # Pipeline oluştur
            self.nlp_pipeline = pipeline("sentiment-analysis", model=model, tokenizer=tokenizer)

            self.text_sentiment_ready = True
            logging.info("Text Sentiment modeli BAŞARIYLA yüklendi.")

        except Exception as e:
            logging.error(f"Text Sentiment modeli yüklenirken hata: {e}")
            # Hata detayını görmek için:
            import traceback
            logging.error(traceback.format_exc())
            self.text_sentiment_ready = False

    def load_speaker_db(self):
        if os.path.exists(SPEAKER_DB_FILE):
            try:
                with open(SPEAKER_DB_FILE, 'r') as f:
                    self.speaker_vectors = json.load(f)
                logging.info(f"Speakers DB yüklendi: {len(self.speaker_vectors)} kullanıcı.")
            except Exception as e:
                logging.error(f"DB okuma hatası: {e}")
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

# Global Model Nesnesi
models = MLModels()

# --- Özellik Çıkarma (Librosa) ---
def extract_features_from_bytes(wav_bytes, sr=None):
    try:
        with io.BytesIO(wav_bytes) as wav_buffer:
            audio, sample_rate = librosa.load(wav_buffer, sr=sr)

        if len(audio) < 1024:
            logging.warning("Ses çok kısa, özellik çıkarımı sağlıksız olabilir.")

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

    except Exception as e:
        logging.error(f"Feature extraction error: {e}")
        raise e

def get_column_names():
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

# --- İŞLEM MANTIĞI ---

def process_sentiment(raw_features):
    """Voice Sentiment (Ses Duygu) modeli için veriyi hazırlar ve tahmin eder."""
    if not models.sentiment_ready:
        return "ModelNotLoaded"

    try:
        columns = get_column_names()
        if len(raw_features) != len(columns):
            logging.warning("Feature size mismatch for sentiment.")
            return "DimMismatch"

        df = pd.DataFrame([raw_features], columns=columns)

        X_selected = df[models.sent_selector].values
        X_scaled = models.sent_scaler.transform(X_selected)

        # CNN/LSTM için reshape (Samples, Time steps, Features)
        X_reshaped = X_scaled.reshape(X_scaled.shape[0], X_scaled.shape[1], 1)

        preds = models.sent_model.predict(X_reshaped, verbose=0)
        pred_class = np.argmax(preds, axis=1)
        label = models.sent_label_encoder.inverse_transform(pred_class)[0]

        return label
    except Exception as e:
        logging.error(f"Sentiment process error: {e}")
        return "Error"

# --- YENİ FONKSİYON: TEXT SENTIMENT ---
def process_text_sentiment(text):
    """Gelen metnin duygu analizini yapar."""
    # Model yüklü değilse veya metin çok kısaysa
    if not models.text_sentiment_ready or not text or len(text.strip()) < 2:
        return "Nötr"

    try:
        # Pipeline çağrısı. Uzun metinlerde hata almamak için truncation=True
        result = models.nlp_pipeline(text, truncation=True, max_length=512)[0]
        label = result['label']

        # Mapping (Modelin çıktısına göre Frontend uyumu)
        if label == "positive":
            return "Olumlu"
        elif label == "negative":
            return "Olumsuz"
        else:
            return "Nötr"

    except Exception as e:
        logging.error(f"Text sentiment processing error: {e}")
        return "Hata"

def process_recognition_vector(raw_features):
    """Recognition için ham özellikleri Recognition klasöründeki scaler/selector ile işler."""
    if not models.recognition_ready:
        logging.error("Recognition modelleri yüklü değil.")
        return raw_features

    try:
        X = raw_features.reshape(1, -1)
        X_scaled = models.rec_scaler.transform(X)
        X_final = models.rec_selector.transform(X_scaled)
        return X_final[0]

    except Exception as e:
        logging.error(f"Recognition vector process error: {e}")
        return raw_features

def identify_speaker_logic(processed_vector):
    """Threshold kontrolü OLMADAN en benzer kullanıcıyı ve skorunu döner."""
    if not models.speaker_vectors:
        return None, 0.0

    best_user = "Unknown"
    best_score = -1.0

    input_vec = processed_vector.reshape(1, -1)

    for user_id, vectors_list in models.speaker_vectors.items():
        db_vectors = np.array(vectors_list)

        if db_vectors.shape[1] != input_vec.shape[1]:
            continue

        similarities = cosine_similarity(input_vec, db_vectors)
        max_sim = np.max(similarities)

        if max_sim > best_score:
            best_score = max_sim
            best_user = user_id

    logging.info(f"En yakın: {best_user} | Skor: {best_score:.4f}")
    return best_user, float(best_score)

# --- API ENDPOINTS ---

@app.route('/identificate', methods=['POST'])
def identificate_user():
    """Kullanıcı kaydı yapar (Speaker Embedding'i veritabanına ekler)."""
    try:
        data = request.json
        if not data or 'speaker' not in data or 'wav_file' not in data:
            return jsonify({"error": "Eksik veri"}), 400

        user_id = data['speaker']
        wav_base64 = data['wav_file']

        wav_bytes = base64.b64decode(wav_base64)
        raw_features = extract_features_from_bytes(wav_bytes)

        processed_vector = process_recognition_vector(raw_features)
        models.add_speaker_vector(user_id, processed_vector)

        return jsonify({"status": "success", "message": f"User {user_id} saved."})

    except Exception as e:
        logging.error(f"Identificate error: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/', methods=['POST'])
def analyze():
    """
    Gelen sesi analiz eder:
    1. Duygu Analizi (Ses)
    2. Konuşmacı Tanıma
    3. Duygu Analizi (Metin) - YENİ
    """
    try:
        data = request.json
        if not data or 'wav_file' not in data:
            return jsonify({"error": "Eksik veri"}), 400

        wav_base64 = data['wav_file']
        wav_bytes = base64.b64decode(wav_base64)

        # Frontend veya Whisper servisinden gelen metin
        input_text = data.get('text', '')

        # 1. Ham Özellikler
        raw_features = extract_features_from_bytes(wav_bytes)

        # 2. Ses Duygu Analizi
        voice_sentiment = process_sentiment(raw_features)

        # 3. Konuşmacı Tanıma
        rec_vector = process_recognition_vector(raw_features)
        speaker_id, speaker_score = identify_speaker_logic(rec_vector)

        # 4. METİN DUYGU ANALİZİ (YENİ)
        text_sentiment_result = process_text_sentiment(input_text)

        response = {
            "segment_id": data.get('segment_id'),
            "text": input_text,

            "voice_sentiment": voice_sentiment,
            "text_sentiment": text_sentiment_result, # Go'ya gönderilecek alan

            "speaker": speaker_id,
            "similarity_score": speaker_score,
            "language": data.get('language', ''),
            "start": data.get('start', 0.0),
            "end": data.get('end', 0.0),
            "status": "success"
        }
        return jsonify(response)

    except Exception as e:
        logging.error(f"Analyze error: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001, debug=True)