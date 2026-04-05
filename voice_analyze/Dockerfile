# voice_analyze/Dockerfile
FROM python:3.12-slim

WORKDIR /app

# Librosa için gerekli sistem bağımlılıkları (Ses işleme araçları)
RUN apt-get update && \
    apt-get install -y ffmpeg libsndfile1 gcc && \
    rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Servis 5001 portunda çalışıyor
EXPOSE 5001

CMD ["python", "app.py"]