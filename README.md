# Gerçek Zamanlı Ses ve Duygu Analizi Sistemi

Bu proje, mikroservis mimarisi kullanılarak geliştirilmiş, gerçek zamanlı konuşma dökümü (STT), konuşmacı ayrımı (diarization), duygu analizi ve konuşmacı tanıma özelliklerine sahip kapsamlı bir ses analiz sistemidir.

## Mimariler ve İşleyiş

### 1. Canlı Ses Analizi (Live Audio Analysis)

Bu modül, canlı bir görüşme sırasında sesin yakalanıp, metne dökülmesi, konuşmacıların ayrıştırılması ve hem metin hem de ses tonu üzerinden duygu analizi yapılmasını sağlar.

![Canlı Ses Analizi Mimarisi](assets/live_analysis_architecture.png)

**Çalışma Mantığı:**
1.  **Frontend (Vite):** Kullanıcı arayüzü üzerinden ses kaydı başlatılır ve ses verisi stream edilir.
2.  **Go Gateway Service:** Gelen ses verisini karşılar ve ilgili servislere yönlendirir.
3.  **WhisperX Service:** Sesi işleyerek metne döker (Speech-to-Text) ve konuşmacı ayrımı (Diarization) yapar. Hangi cümlenin kim tarafından söylendiğini belirler.
4.  **Text Analysis Service:** Elde edilen metin üzerinde duygu analizi (Pozitif, Negatif, Nötr) yapar.
5.  **Voice Analysis Service:** Ses segmentlerini analiz ederek konuşmacının o anki duygu durumunu (Örn: Kızgın, Mutlu) ve kimliğini tespit eder.
6.  **Sonuç:** Tüm analiz sonuçları birleştirilerek anlık olarak Frontend'e iletilir ve tabloda gösterilir.

**Ekran Görüntüsü:**
Aşağıda canlı analiz ekranının bir örneği görülmektedir.
![Canlı Ses Analizi Ekranı](assets/live_analysis_ui.png)

---

### 2. Kullanıcı Kaydı (User Registration)

Sistemin konuşmacıları tanıması için (Speaker Identification), kullanıcıların ses örnekleriyle sisteme kayıt olması gerekmektedir.

![Kullanıcı Kayıt Mimarisi](assets/user_registration_architecture.png)

**Çalışma Mantığı:**
1.  **Frontend:** Kullanıcı Ad, Soyad bilgilerini girer ve belirtilen metni sesli olarak okur.
2.  **Go Gateway:** Form verisini ve ses dosyasını alır.
    *   Kullanıcı meta verilerini (Ad, Soyad vb.) **Main DB (Speaker Meta)** veritabanına kaydeder.
    *   Ses dosyasını analiz servisine iletir.
3.  **Voice Analyze Service:**
    *   Gelen sesten öznitelik çıkarımı (Feature Extraction) yapar.
    *   Kullanıcının ses imzasını (vektörünü) **Voice DB (Embeddings)** veritabanına kaydeder.
4.  **Sonuç:** Kayıt işlemi tamamlandığında kullanıcıya başarı mesajı döner.

**Ekran Görüntüleri:**
Kullanıcı kayıt formu ve kayıtlı kişiler listesi aşağıdadır.

*Yeni Kişi Kaydı:*
![Yeni Kişi Kaydı](assets/user_registration_ui.png)

*Kayıtlı Kişiler Listesi:*
![Kayıtlı Kişiler](assets/user_list_ui.png)
