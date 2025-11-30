import os
import numpy as np
import librosa
import joblib
from tpot import TPOTClassifier
from sklearn.preprocessing import StandardScaler, LabelEncoder
from sklearn.svm import SVC

class SpeakerRecognition:
    def __init__(self, model_dir='models_speaker'):
        # model_dir relative to this file
        base_dir = os.path.dirname(os.path.abspath(__file__))
        self.model_dir = os.path.join(base_dir, model_dir)

        if not os.path.exists(self.model_dir):
            os.makedirs(self.model_dir)

        self.tpot_path = os.path.join(self.model_dir, 'tpot_best.pkl')
        self.svm_path = os.path.join(self.model_dir, 'svm_best.pkl')
        self.scaler_path = os.path.join(self.model_dir, 'scaler.pkl')
        self.le_path = os.path.join(self.model_dir, 'label_encoder.pkl')

        self.tpot_model = None
        self.svm_model = None
        self.scaler = None
        self.label_encoder = None

        self.load_models()

    def load_models(self):
        try:
            if os.path.exists(self.tpot_path):
                self.tpot_model = joblib.load(self.tpot_path)
            if os.path.exists(self.svm_path):
                self.svm_model = joblib.load(self.svm_path)
            if os.path.exists(self.scaler_path):
                self.scaler = joblib.load(self.scaler_path)
            if os.path.exists(self.le_path):
                self.label_encoder = joblib.load(self.le_path)
            print("Speaker Recognition models loaded.")
        except Exception as e:
            print(f"Error loading models: {e}")

    def extract_features(self, audio, sample_rate):
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
        return features

    def train(self, file_list):
        # file_list: list of dicts with 'path' and 'name'
        X = []
        y = []

        base_dir = os.path.dirname(os.path.abspath(__file__))

        for item in file_list:
            path = item['path']
            name = item['name']
            try:
                # Handle path. path comes from Go, likely relative to Go root or absolute.
                # If relative, it might be 'record_matches/...'
                # We need to find it.

                real_path = path
                if not os.path.exists(real_path):
                     # Try finding it relative to one level up (if running in analyze/)
                     candidate = os.path.join(base_dir, '..', path)
                     if os.path.exists(candidate):
                         real_path = candidate

                if not os.path.exists(real_path):
                    print(f"File not found: {path} or {real_path}")
                    continue

                audio, sr = librosa.load(real_path, sr=None)
                features = self.extract_features(audio, sr)
                X.append(features)
                y.append(name)
            except Exception as e:
                print(f"Error processing {path}: {e}")

        if not X:
            return False, "No valid audio files found."

        if len(set(y)) < 2:
            return False, f"Not enough classes to train. Found: {len(set(y))}. Need at least 2."

        X = np.array(X)
        y = np.array(y)

        # Encode labels
        self.label_encoder = LabelEncoder()
        y_int = self.label_encoder.fit_transform(y)
        joblib.dump(self.label_encoder, self.le_path)

        # Scale
        self.scaler = StandardScaler()
        X_scaled = self.scaler.fit_transform(X)
        joblib.dump(self.scaler, self.scaler_path)

        # Train TPOT
        print("Training TPOT...")
        tpot = TPOTClassifier(
            generations=5,
            population_size=10,
            random_state=42,
            max_time_mins=2,
            n_jobs=1
        )
        tpot.fit(X_scaled, y_int)
        self.tpot_model = tpot.fitted_pipeline_
        joblib.dump(self.tpot_model, self.tpot_path)

        # Train SVM
        print("Training SVM...")
        svm = SVC(kernel="rbf", probability=True, random_state=42)
        svm.fit(X_scaled, y_int)
        self.svm_model = svm
        joblib.dump(self.svm_model, self.svm_path)

        return True, "Training completed."

    def predict(self, audio, sample_rate):
        if not self.tpot_model or not self.scaler or not self.label_encoder:
            return "Unknown"

        try:
            features = self.extract_features(audio, sample_rate)
            features = features.reshape(1, -1)
            features_scaled = self.scaler.transform(features)

            # Simple prediction with TPOT
            pred_int = self.tpot_model.predict(features_scaled)
            pred_label = self.label_encoder.inverse_transform(pred_int)
            return pred_label[0]

        except Exception as e:
            print(f"Prediction error: {e}")
            return "Error"
