import os
import logging
import traceback
from flask import Flask, request, jsonify
from transformers import pipeline, AutoTokenizer, TFAutoModelForSequenceClassification
import google.generativeai as genai

# Loglama ayarları
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

app = Flask(__name__)

# --- GEMINI AYARLARI ---
# API Key'i ortam değişkenlerinden alıyoruz.
GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY")

if GEMINI_API_KEY:
    genai.configure(api_key=GEMINI_API_KEY)
    logging.info("Gemini API Key başarıyla yapılandırıldı.")
else:
    logging.warning("UYARI: GEMINI_API_KEY ortam değişkeni bulunamadı! Konu analizi çalışmayacaktır.")

# Dizin Ayarları
BASE_DIR = os.path.dirname(os.path.abspath(__file__))

class TextModel:
    def __init__(self):
        logging.info("Text Service başlatılıyor...")
        self.ready = False
        self.load_model()

    def load_model(self):
        """Yerel klasörden Türkçe Metin Duygu Analizi modelini yükler."""
        try:
            model_path = os.path.join(BASE_DIR, 'models')
            logging.info(f"Text Sentiment modeli yükleniyor: {model_path}")

            tokenizer = AutoTokenizer.from_pretrained(model_path)
            model = TFAutoModelForSequenceClassification.from_pretrained(model_path)

            self.nlp_pipeline = pipeline("sentiment-analysis", model=model, tokenizer=tokenizer)
            self.ready = True
            logging.info("Text Sentiment modeli BAŞARIYLA yüklendi.")

        except Exception as e:
            logging.error(f"Text Sentiment modeli yüklenirken hata: {e}")
            logging.error(traceback.format_exc())
            self.ready = False

# Global Model Nesnesi
text_model = TextModel()

def process_text_sentiment(text):
    if not text_model.ready or not text or len(text.strip()) < 2:
        return "Nötr"

    try:
        # Uzun metinlerde hata almamak için truncation=True
        result = text_model.nlp_pipeline(text, truncation=True, max_length=512)[0]
        label = result['label']

        if label == "positive":
            return "Olumlu"
        elif label == "negative":
            return "Olumsuz"
        else:
            return "Nötr"
    except Exception as e:
        logging.error(f"Processing error: {e}")
        return "Hata"

def determine_topic_llm(text):
    """
    Google Gemini modelini kullanarak metinden konu başlığı çıkarır.
    """
    if not GEMINI_API_KEY:
        return "API Key Eksik"

    if not text or len(text) < 5:
        return "Kısa Konuşma"

    try:
        model = genai.GenerativeModel('models/gemini-2.5-flash')

        prompt = f"""
        Aşağıdaki konuşma metnini analiz et ve bu konuşmaya uygun, 
        kısa (maksimum 3-5 kelime) Türkçe bir konu başlığı ver.
        Sadece başlığı yaz, açıklama yapma.
        
        Metin:
        {text}
        """

        # generate_content metodu çağrılıyor
        response = model.generate_content(prompt)

        # Yanıt metnini al ve temizle
        if response.text:
            topic = response.text.strip().replace('"', '').replace("'", "").replace("**", "")
            return topic
        else:
            return "Konu Belirlenemedi"

    except Exception as e:
        logging.error(f"Gemini API Hatası: {e}")
        # Hata detayını loglarda görmek için traceback ekleyebiliriz
        logging.error(traceback.format_exc())
        return "Genel Konuşma (API Hatası)"

@app.route('/sentiment', methods=['POST'])
def analyze_text():
    try:
        data = request.json
        text = data.get('text', '')

        sentiment = process_text_sentiment(text)

        return jsonify({
            "status": "success",
            "text": text,
            "sentiment": sentiment
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/analyze_topic', methods=['POST'])
def analyze_topic():
    """
    Konuşmanın tamamını alır ve Gemini ile konu başlığı üretir.
    """
    try:
        data = request.json
        text = data.get('text', '')

        logging.info(f"Topic Analysis İsteği Alındı. Metin uzunluğu: {len(text)}")

        topic = determine_topic_llm(text)

        logging.info(f"Topic Result: {topic}")

        return jsonify({
            "status": "success",
            "topic": topic
        })
    except Exception as e:
        logging.error(f"Topic analysis error: {e}")
        return jsonify({"error": str(e), "topic": "Analiz Hatası"}), 500

if __name__ == '__main__':
    # Bu servis 5002 portunda çalışacak
    app.run(host='0.0.0.0', port=5002, debug=False)