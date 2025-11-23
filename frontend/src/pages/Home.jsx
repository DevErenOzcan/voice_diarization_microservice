import React, { useState, useRef, useEffect } from 'react';
import { useWebSocket } from '../context/WebSocketContext';
import { floatTo16BitPCM } from '../utils/audioUtils';
import { Mic, Square, Activity, Clock, User, MessageSquare } from 'lucide-react';

const Home = () => {
    const { socketRef, lastMessage } = useWebSocket();
    const [isRecording, setIsRecording] = useState(false);

    // Tek bir obje yerine artık bir liste tutuyoruz
    const [segments, setSegments] = useState([]);

    // Otomatik scroll için referans
    const tableEndRef = useRef(null);

    const audioContextRef = useRef(null);
    const processorRef = useRef(null);
    const inputRef = useRef(null);

    // Backend'den gelen yeni segmenti listeye ekle
    useEffect(() => {
        if (lastMessage && lastMessage.type === 'live_analysis') {
            setSegments((prevSegments) => [...prevSegments, lastMessage.payload]);
        }
    }, [lastMessage]);

    // Her yeni veri geldiğinde tablonun en altına kaydır
    useEffect(() => {
        tableEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [segments]);

    const startRecording = async () => {
        try {
            // Önceki kayıtları temizlemek istersen bu satırı açabilirsin:
            // setSegments([]);

            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

            const audioContext = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: 16000,
            });
            audioContextRef.current = audioContext;

            const source = audioContext.createMediaStreamSource(stream);
            inputRef.current = source;

            const bufferSize = 4096;
            const processor = audioContext.createScriptProcessor(bufferSize, 1, 1);
            processorRef.current = processor;

            processor.onaudioprocess = (e) => {
                if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) return;
                const inputData = e.inputBuffer.getChannelData(0);
                const pcmData = floatTo16BitPCM(inputData);
                socketRef.current.send(pcmData);
            };

            source.connect(processor);
            processor.connect(audioContext.destination);

            setIsRecording(true);
        } catch (err) {
            console.error("Mikrofon hatası:", err);
            alert("Mikrofon izni alınamadı.");
        }
    };

    const stopRecording = () => {
        if (processorRef.current) {
            processorRef.current.disconnect();
            processorRef.current.onaudioprocess = null;
        }
        if (inputRef.current) inputRef.current.disconnect();
        if (audioContextRef.current) audioContextRef.current.close();

        setIsRecording(false);
    };

    // Saniye cinsinden gelen süreyi (örn: 1.25) formata çevirir (00:01)
    const formatTime = (seconds) => {
        if (!seconds && seconds !== 0) return "-";
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    };

    return (
        <div className="p-6 max-w-6xl mx-auto h-[calc(100vh-100px)] flex flex-col">

            {/* Üst Kısım: Başlık ve Kayıt Butonu */}
            <div className="text-center mb-8 flex-shrink-0">
                <h1 className="text-3xl font-bold text-gray-800 mb-2">Canlı Görüşme Analizi</h1>
                <p className="text-gray-500 mb-6">Görüşmeyi başlatmak için aşağıdaki butona basın.</p>

                <button
                    onClick={isRecording ? stopRecording : startRecording}
                    className={`relative z-10 p-8 rounded-full transition-all duration-300 shadow-2xl flex items-center justify-center mx-auto
            ${isRecording ? 'bg-red-500 hover:bg-red-600 recording-animation' : 'bg-indigo-600 hover:bg-indigo-700'}`}
                >
                    {isRecording ? <Square size={40} className="text-white" /> : <Mic size={40} className="text-white" />}
                </button>

                {isRecording && (
                    <div className="mt-4 flex items-center justify-center gap-2 text-red-500 font-semibold animate-pulse">
                        <Activity size={18} />
                        <span>Kayıt Yapılıyor ve Analiz Ediliyor...</span>
                    </div>
                )}
            </div>

            {/* Alt Kısım: Anlık Veri Tablosu */}
            <div className="flex-grow bg-white rounded-xl shadow-lg border border-gray-200 flex flex-col overflow-hidden">
                {/* Tablo Başlığı */}
                <div className="bg-gray-50 px-6 py-3 border-b border-gray-200 flex justify-between items-center">
                    <h2 className="font-bold text-gray-700 flex items-center gap-2">
                        <MessageSquare size={18}/> Segment Analizi
                    </h2>
                    <span className="text-xs bg-indigo-100 text-indigo-800 px-2 py-1 rounded-full">
            Toplam {segments.length} Segment
          </span>
                </div>

                {/* Tablo İçeriği (Scroll edilebilir alan) */}
                <div className="overflow-y-auto flex-grow p-0">
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50 sticky top-0 z-10 shadow-sm">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider w-24">
                                <div className="flex items-center gap-1"><Clock size={14}/> Süre</div>
                            </th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider w-32">
                                <div className="flex items-center gap-1"><User size={14}/> Konuşmacı</div>
                            </th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                                Metin
                            </th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider w-32">
                                Duygu (Metin)
                            </th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider w-32">
                                Duygu (Ses)
                            </th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {segments.length === 0 ? (
                            <tr>
                                <td colSpan="5" className="px-6 py-12 text-center text-gray-400">
                                    Henüz analiz verisi yok. Kaydı başlatın ve konuşun...
                                </td>
                            </tr>
                        ) : (
                            segments.map((seg, index) => (
                                <tr key={index} className="hover:bg-indigo-50 transition-colors">
                                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 font-mono">
                                        {formatTime(seg.start)} - {formatTime(seg.end)}
                                    </td>
                                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium 
                        ${seg.speaker === 'Bilinmiyor' ? 'bg-gray-100 text-gray-800' : 'bg-blue-100 text-blue-800'}`}>
                        {seg.speaker || 'Bilinmiyor'}
                      </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-900">
                                        {seg.text}
                                    </td>
                                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                                        {seg.textSentiment ? (
                                            <span className={`px-2 py-1 rounded text-xs font-semibold
                          ${seg.textSentiment.toLowerCase().includes('pos') ? 'bg-green-100 text-green-800' :
                                                seg.textSentiment.toLowerCase().includes('neg') ? 'bg-red-100 text-red-800' : 'bg-yellow-100 text-yellow-800'}`}>
                          {seg.textSentiment}
                        </span>
                                        ) : '-'}
                                    </td>
                                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                                        {seg.voiceSentiment ? (
                                            <span className="text-purple-700 font-medium text-xs border border-purple-200 px-2 py-1 rounded">
                          {seg.voiceSentiment}
                        </span>
                                        ) : '-'}
                                    </td>
                                </tr>
                            ))
                        )}
                        {/* Otomatik scroll için görünmez eleman */}
                        <div ref={tableEndRef} />
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
};

export default Home;