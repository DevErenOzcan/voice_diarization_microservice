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
from speaker_recognition import SpeakerRecognition

# Loglama ayarları
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = Flask(__name__)

# --- Modelleri Yükleme ---
class MLModels:
    def __init__(self):
        logging.info("Modeller yükleniyor...")
        try:
            # GPU kullanımını kapat
            tf.config.set_visible_devices([], 'GPU')

            base_dir = os.path.dirname(os.path.abspath(__file__))
            env_model_dir = os.getenv('ANALYZE_MODEL_DIR')
            if env_model_dir:
                if not os.path.isabs(env_model_dir):
                    model_dir = os.path.join(base_dir, env_model_dir)
                else:
                    model_dir = env_model_dir
                logging.info(f"ANALYZE_MODEL_DIR provided, using: {model_dir}")
            else:
                model_dir = os.path.join(base_dir, './../models/old')
                logging.info(f"ANALYZE_MODEL_DIR not set, defaulting to: {model_dir}")

            scaler_path = os.path.join(model_dir, 'scaler.pkl')
            selector_path = os.path.join(model_dir, 'selector.pkl')
            label_encoder_path = os.path.join(model_dir, 'label_encoder.pkl')
            best_model_path = os.path.join(model_dir, 'best_model.keras')

            # Check files
            missing = [p for p in (scaler_path, selector_path, label_encoder_path, best_model_path) if not os.path.exists(p)]
            if missing:
                logging.error("Aşağıdaki model dosyaları bulunamadı:")
                for m in missing:
                    logging.error(f"  - {m}")
                self.sentiment_models_loaded = False
            else:
                self.scaler = load(scaler_path)
                self.selector = load(selector_path)
                self.label_encoder = load(label_encoder_path)
                self.best_model = tf.keras.models.load_model(best_model_path)
                self.sentiment_models_loaded = True
                logging.info("Sentiment modelleri başarıyla yüklendi.")
        except Exception as e:
            logging.error(f"Model yükleme hatası: {e}")
            self.sentiment_models_loaded = False

# Modelleri başlat
models = MLModels()
speaker_recognition = SpeakerRecognition()

# --- Yardımcı Fonksiyonlar ---

def get_column_names():
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
    try:
        with io.BytesIO(wav_bytes) as wav_buffer:
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

        features = np.hstack([
            zero_crossing, spectral_centroid, spectral_rolloff, spectral_bandwidth,
            contrast_mean, contrast_std, chroma_stft_mean, chroma_stft_std,
            rms_mean, melspectrogram_mean, melspectrogram_std, flatness_mean,
            poly_mean, mfcc_mean, mfcc_std, energy
        ])

        return features, audio, sample_rate

    except Exception as e:
        logging.error(f"Öznitelik çıkarma hatası: {e}")
        raise e

def predict_sentiment(df_features):
    if not models.sentiment_models_loaded:
        return "ModelNotLoaded"
    try:
        new_features = df_features[models.selector].values
        new_features_scaled = models.scaler.transform(new_features)
        new_features_scaled = new_features_scaled.reshape(new_features_scaled.shape[0], new_features_scaled.shape[1], 1)
        predictions = models.best_model.predict(new_features_scaled, verbose=0)
        predicted_classes = np.argmax(predictions, axis=1)
        predicted_labels = models.label_encoder.inverse_transform(predicted_classes)
        return predicted_labels[0]
    except Exception as e:
        logging.error(f"Tahminleme hatası: {e}")
        return "Error"

@app.route('/', methods=['POST'])
def analyze():
    try:
        data = request.json
        if not data:
            return jsonify({"error": "Veri bulunamadı"}), 400

        segment_id = data.get('segment_id')
        wav_base64 = data.get('wav_file')
        text = data.get('text', '')
        language = data.get('language', '')
        start = data.get('start', 0.0)
        end = data.get('end', 0.0)

        if not wav_base64:
            return jsonify({"error": "wav_file eksik"}), 400

        wav_bytes = base64.b64decode(wav_base64)
        features, audio, sample_rate = extract_features_from_bytes(wav_bytes, sr=16000)
        columns = get_column_names()

        if len(features) != len(columns):
            logging.warning(f"Feature sayısı uyuşmazlığı! Beklenen: {len(columns)}, Çıkan: {len(features)}")

        df = pd.DataFrame([features], columns=columns)

        audio_sentiment = predict_sentiment(df)

        # Predict Speaker
        speaker = speaker_recognition.predict(audio, sample_rate)

        logging.info(f"Segment: {segment_id} | Tahmin: {audio_sentiment} | Speaker: {speaker}")

        response = {
            "segment_id": segment_id,
            "text": text,
            "voice_sentiment": audio_sentiment,
            "speaker": speaker,
            "language": language,
            "start": start,
            "end": end,
            "status": "success"
        }

        return jsonify(response)

    except Exception as e:
        logging.error(f"API Hatası: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/train_recognition_model', methods=['POST'])
def train_recognition_model():
    try:
        data = request.json
        files = data.get('files', []) # List of {path, name}
        if not files:
            return jsonify({"error": "No files provided"}), 400

        logging.info(f"Starting training with {len(files)} files...")

        success, message = speaker_recognition.train(files)

        if success:
             logging.info("Training completed successfully.")
             return jsonify({"status": "success", "message": message})
        else:
             logging.error(f"Training failed: {message}")
             return jsonify({"status": "error", "message": message}), 500

    except Exception as e:
        logging.error(f"Training API Error: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001, debug=True)
