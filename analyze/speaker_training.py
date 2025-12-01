import os
import pandas as pd
import numpy as np
import joblib
import librosa
import logging
from tpot import TPOTClassifier
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import LabelEncoder

# Set up logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

def extract_features(file_path):
    try:
        audio, sample_rate = librosa.load(file_path, sr=None)

        zero_crossing = np.mean(librosa.feature.zero_crossing_rate(y=audio).T, axis=0)
        spectral_centroid = np.mean(librosa.feature.spectral_centroid(y=audio, sr=sample_rate).T, axis=0)
        spectral_rolloff  = np.mean(librosa.feature.spectral_rolloff(y=audio, sr=sample_rate).T, axis=0)
        spectral_bandwidth = np.mean(librosa.feature.spectral_bandwidth(y=audio, sr=sample_rate).T, axis=0)

        spectral_contrast = librosa.feature.spectral_contrast(y=audio, sr=sample_rate)
        contrast_mean = np.mean(spectral_contrast, axis=1)
        contrast_std  = np.std(spectral_contrast, axis=1)

        chroma_stft = librosa.feature.chroma_stft(y=audio, sr=sample_rate)
        chroma_stft_mean = np.mean(chroma_stft, axis=1)
        chroma_stft_std  = np.std(chroma_stft, axis=1)

        rms_mean = np.mean(librosa.feature.rms(y=audio))

        mel_spectrogram = librosa.feature.melspectrogram(y=audio, sr=sample_rate)
        melspectrogram_mean = np.mean(mel_spectrogram)
        melspectrogram_std  = np.std(mel_spectrogram)

        flatness_mean = np.mean(librosa.feature.spectral_flatness(y=audio))

        poly_features = librosa.feature.poly_features(y=audio, sr=sample_rate, order=1)
        poly_mean = np.mean(poly_features, axis=1)

        mfcc = librosa.feature.mfcc(y=audio, sr=sample_rate, n_mfcc=40)
        mfcc_mean = np.mean(mfcc, axis=1)
        mfcc_std  = np.std(mfcc, axis=1)

        energy = np.sum(audio ** 2)

        features = np.hstack([
            zero_crossing, spectral_centroid, spectral_rolloff, spectral_bandwidth,
            contrast_mean, contrast_std, chroma_stft_mean, chroma_stft_std,
            rms_mean, melspectrogram_mean, melspectrogram_std, flatness_mean,
            poly_mean, mfcc_mean, mfcc_std, energy
        ])
        return features
    except Exception as e:
        logging.error(f"Error processing {file_path}: {e}")
        return None

def train_speaker_model():
    logging.info("Starting speaker model training...")

    # 1. Read Data
    records_path = os.path.join(os.path.dirname(__file__), 'records.csv')
    if not os.path.exists(records_path):
        logging.error("No records.csv found. Cannot train.")
        return

    df = pd.read_csv(records_path)
    if df.empty:
        logging.error("records.csv is empty. Cannot train.")
        return

    # Extract features
    features_list = []
    labels = []

    logging.info("Extracting features from audio files...")
    for idx, row in df.iterrows():
        f_path = row['file_path']
        user_id = row['user_id']

        # Ensure path is absolute or correct relative to here
        if not os.path.exists(f_path):
            # Check if relative to analyze dir
            alt_path = os.path.join(os.path.dirname(__file__), f_path)
            if os.path.exists(alt_path):
                f_path = alt_path
            else:
                logging.warning(f"File not found: {f_path}, skipping.")
                continue

        feat = extract_features(f_path)
        if feat is not None:
            features_list.append(feat)
            labels.append(str(user_id))

    if not features_list:
        logging.error("No valid audio files processed.")
        return

    X = np.array(features_list)
    y_text = np.array(labels)

    logging.info(f"Training on {len(X)} samples, {len(set(y_text))} classes.")

    # 2. Split
    if len(X) < 2:
        logging.warning("Not enough samples to split. Training on all data.")
        X_train = X
        y_train_txt = y_text
    else:
        try:
            idx_all = np.arange(len(X))
            # Try stratified split
            idx_train, idx_test = train_test_split(
                idx_all, test_size=0.20, random_state=42, stratify=y_text
            )
            X_train = X[idx_train]
            y_train_txt = y_text[idx_train]
        except ValueError:
            # Fallback if cannot stratify
            logging.warning("Cannot stratify split. Using random split.")
            try:
                idx_train, idx_test = train_test_split(
                    idx_all, test_size=0.20, random_state=42
                )
                X_train = X[idx_train]
                y_train_txt = y_text[idx_train]
            except ValueError:
                 X_train = X
                 y_train_txt = y_text

    # 3. Encode Labels
    le = LabelEncoder().fit(y_text)

    # 4. Train TPOT
    logging.info("Fitting TPOT...")
    # Adjust CV for small datasets
    cv_val = 5
    min_class_samples = pd.Series(y_train_txt).value_counts().min()
    if min_class_samples < 5:
        cv_val = min(min_class_samples, 2)
        if cv_val < 2:
            cv_val = 2 # At least 2 for CV, otherwise standard split

    # Removed config_dict as it is not available in installed TPOT version
    tpot = TPOTClassifier(
        generations=30, population_size=50, scoring="f1_macro", cv=cv_val,
        n_jobs=-1, verbosity=2, random_state=42, max_time_mins=100
    )

    try:
        tpot.fit(X_train, y_train_txt)
    except Exception as e:
        logging.error(f"TPOT training failed: {e}")
        return

    # Save
    models_dir = os.path.join(os.path.dirname(__file__), 'models', 'current')
    if not os.path.exists(models_dir):
        os.makedirs(models_dir)

    joblib.dump(tpot.fitted_pipeline_, os.path.join(models_dir, "tpot_best.pkl"))
    joblib.dump(le, os.path.join(models_dir, "label_encoder.pkl"))

    logging.info("Training complete and models saved to models/current/.")

if __name__ == "__main__":
    train_speaker_model()
