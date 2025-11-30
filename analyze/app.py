import os
import io
import base64
import logging
import numpy as np
import pandas as pd
import librosa
import tensorflow as tf
import csv   # EKLENDİ
import time  # EKLENDİ
from flask import Flask, request, jsonify
from joblib import load

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
                model_dir = os.path.join(base_dir, './models/old')
                logging.info(f"ANALYZE_MODEL_DIR not set, defaulting to: {model_dir}")

            scaler_path = os.path.join(model_dir, 'scaler.pkl')
            selector_path = os.path.join(model_dir, 'selector.pkl')
            label_encoder_path = os.path.join(model_dir, 'label_encoder.pkl')
            best_model_path = os.path.join(model_dir, 'best_model.keras')

            logging.info(f"Model dizini: {model_dir}")

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
        logging.error(f"Öznitelik çıkarma hatası: {e}")
        raise e

def predict_sentiment(df_features):
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

# --- Endpoints ---

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
        features = extract_features_from_bytes(wav_bytes, sr=16000)
        columns = get_column_names()

        if len(features) != len(columns):
            logging.warning(f"Feature sayısı uyuşmazlığı! Beklenen: {len(columns)}, Çıkan: {len(features)}")

        df = pd.DataFrame([features], columns=columns)
        audio_sentiment = predict_sentiment(df)

        logging.info(f"Segment: {segment_id} | Tahmin: {audio_sentiment} | Start: {start}")

        response = {
            "segment_id": segment_id,
            "text": text,
            "voice_sentiment": audio_sentiment,
            "language": language,
            "start": start,
            "end": end,
            "status": "success"
        }

        return jsonify(response)

    except Exception as e:
        logging.error(f"API Hatası: {e}")
        return jsonify({"error": str(e)}), 500

# --- YENİ EKLENEN ENDPOINT ---
@app.route('/identificate', methods=['POST'])
def identificate_user():
    """
    Kullanıcı kaydı oluşturulduğunda ses dosyasını kaydeder ve CSV'ye işler.
    Go tarafından gönderilen JSON: { "speaker": "USER_ID", "wav_file": "BASE64..." }
    """
    try:
        data = request.json
        if not data:
            return jsonify({"error": "Veri bulunamadı"}), 400

        user_id = data.get('speaker')  # Go tarafında 'speaker' alanına UserID basılıyor
        wav_base64 = data.get('wav_file')

        if not user_id or not wav_base64:
            return jsonify({"error": "speaker veya wav_file eksik"}), 400

        # 1. Klasör Kontrolü / Oluşturma
        records_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'identification_records')
        if not os.path.exists(records_dir):
            os.makedirs(records_dir)
            logging.info(f"Klasör oluşturuldu: {records_dir}")

        # 2. Dosya İsmi Oluşturma (Çakışmayı önlemek için timestamp kullanıyoruz)
        timestamp = int(time.time())
        filename = f"user_{user_id}_{timestamp}.wav"
        file_path = os.path.join(records_dir, filename)

        # 3. Ses Dosyasını Kaydetme
        try:
            wav_bytes = base64.b64decode(wav_base64)
            with open(file_path, "wb") as f:
                f.write(wav_bytes)
            logging.info(f"Ses dosyası kaydedildi: {file_path}")
        except Exception as file_err:
            logging.error(f"Dosya yazma hatası: {file_err}")
            return jsonify({"error": "Dosya diske yazılamadı"}), 500

        # 4. CSV Dosyasına Ekleme
        csv_file_path = os.path.join('analyze/records.csv')
        file_exists = os.path.isfile(csv_file_path)

        try:
            with open(csv_file_path, mode='a', newline='', encoding='utf-8') as csv_file:
                fieldnames = ['user_id', 'file_path']
                writer = csv.DictWriter(csv_file, fieldnames=fieldnames)

                # Dosya yeni oluşturuluyorsa başlıkları yaz
                if not file_exists:
                    writer.writeheader()

                writer.writerow({'user_id': user_id, 'file_path': file_path})
                logging.info(f"CSV güncellendi: {user_id} -> {file_path}")

        except Exception as csv_err:
            logging.error(f"CSV yazma hatası: {csv_err}")
            # Ses dosyası kaydedildi ama CSV yazılamadıysa, hata dönmek yerine warning basabiliriz
            # veya işlemi başarısız sayabiliriz. Şimdilik hata dönüyoruz.
            return jsonify({"error": "CSV kaydı yapılamadı"}), 500

        return jsonify({"status": "success", "file_path": file_path})

    except Exception as e:
        logging.error(f"Identificate API Hatası: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001, debug=True)