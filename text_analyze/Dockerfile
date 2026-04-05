# text_analyze/Dockerfile
FROM python:3.12-slim

WORKDIR /app

# Sistem bağımlılıklarını kur
RUN apt-get update && apt-get install -y gcc && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Servis 5002 portunda çalışıyor
EXPOSE 5002

CMD ["python", "app.py"]