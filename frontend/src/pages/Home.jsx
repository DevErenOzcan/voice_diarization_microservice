import React, { useState, useRef, useEffect } from 'react';
import { Mic, Square, Activity, MessageSquare } from 'lucide-react';

// audioUtils.js içeriği buraya taşındı
const floatTo16BitPCM = (input) => {
    const output = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
        const s = Math.max(-1, Math.min(1, input[i]));
        output[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
    }
    return output;
};

const Home = () => {
    const [isRecording, setIsRecording] = useState(false);
    const [segments, setSegments] = useState([]);
    const [connectionStatus, setConnectionStatus] = useState('disconnected'); // disconnected, connecting, connected

    const socketRef = useRef(null);
    const audioContextRef = useRef(null);
    const processorRef = useRef(null);
    const inputRef = useRef(null);
    const tableEndRef = useRef(null);

    const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws';

    useEffect(() => {
        tableEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [segments]);

    const startLiveAnalysis = async () => {
        try {
            setConnectionStatus('connecting');
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

            socketRef.current = new WebSocket(WS_URL);

            socketRef.current.onopen = () => {
                console.log('WS Bağlandı, ses işleme başlıyor...');
                setConnectionStatus('connected');
                setIsRecording(true);
                setSegments([]);
                setupAudioProcessing(stream);
            };

            socketRef.current.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    if (data.type === 'live_analysis') {
                        setSegments(prev => [...prev, data.payload]);
                    }
                } catch (e) {
                    console.error("JSON parse hatası:", e);
                }
            };

            socketRef.current.onclose = () => {
                console.log('WS Bağlantısı kapandı');
                setConnectionStatus('disconnected');
                setIsRecording(false);
                stopAudioProcessing();
            };

        } catch (err) {
            console.error("Başlatma hatası:", err);
            alert("Mikrofon izni alınamadı veya sunucuya bağlanılamadı.");
            setConnectionStatus('disconnected');
        }
    };

    const setupAudioProcessing = (stream) => {
        const audioContext = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 16000 });
        audioContextRef.current = audioContext;

        const source = audioContext.createMediaStreamSource(stream);
        inputRef.current = source;

        // ScriptProcessor: 4096 buffer size
        const processor = audioContext.createScriptProcessor(4096, 1, 1);
        processorRef.current = processor;

        processor.onaudioprocess = (e) => {
            if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) return;

            const inputData = e.inputBuffer.getChannelData(0);
            // Burada yukarıda tanımladığımız local fonksiyonu kullanıyoruz
            const pcmData = floatTo16BitPCM(inputData);
            socketRef.current.send(pcmData);
        };

        source.connect(processor);
        processor.connect(audioContext.destination);
    };

    const stopLiveAnalysis = () => {
        if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
            socketRef.current.send("STOP");
            setIsRecording(false);
            stopAudioProcessing();
        }
    };

    const stopAudioProcessing = () => {
        if (processorRef.current) {
            processorRef.current.disconnect();
            processorRef.current.onaudioprocess = null;
        }
        if (inputRef.current) inputRef.current.disconnect();
        if (audioContextRef.current) audioContextRef.current.close();
    };

    const formatTime = (seconds) => {
        if (!seconds && seconds !== 0) return "-";
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    };

    return (
        <div className="p-6 max-w-6xl mx-auto h-[calc(100vh-100px)] flex flex-col">
            <div className="text-center mb-8 flex-shrink-0">
                <h1 className="text-3xl font-bold text-gray-800 mb-2">Canlı Görüşme Analizi</h1>
                <p className="text-gray-500 mb-6">
                    {connectionStatus === 'connecting' ? 'Sunucuya bağlanılıyor...' : 'Görüşmeyi başlatmak için aşağıdaki butona basın.'}
                </p>

                <button
                    onClick={isRecording ? stopLiveAnalysis : startLiveAnalysis}
                    disabled={connectionStatus === 'connecting'}
                    className={`relative z-10 p-8 rounded-full transition-all duration-300 shadow-2xl flex items-center justify-center mx-auto
                        ${isRecording ? 'bg-red-500 hover:bg-red-600 recording-animation' : 'bg-indigo-600 hover:bg-indigo-700'}
                        ${connectionStatus === 'connecting' ? 'opacity-50 cursor-not-allowed' : ''}`}
                >
                    {isRecording ? <Square size={40} className="text-white" /> : <Mic size={40} className="text-white" />}
                </button>

                {isRecording && (
                    <div className="mt-4 flex items-center justify-center gap-2 text-red-500 font-semibold animate-pulse">
                        <Activity size={18} />
                        <span>Canlı Analiz Aktif</span>
                    </div>
                )}
            </div>

            <div className="flex-grow bg-white rounded-xl shadow-lg border border-gray-200 flex flex-col overflow-hidden">
                <div className="bg-gray-50 px-6 py-3 border-b border-gray-200 flex justify-between items-center">
                    <h2 className="font-bold text-gray-700 flex items-center gap-2">
                        <MessageSquare size={18}/> Segment Analizi
                    </h2>
                    <span className="text-xs bg-indigo-100 text-indigo-800 px-2 py-1 rounded-full">
                        {segments.length} Segment
                    </span>
                </div>

                <div className="overflow-y-auto flex-grow p-0">
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50 sticky top-0 z-10 shadow-sm">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-24">Süre</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-32">Konuşmacı</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase">Metin</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-32">Duygu</th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {segments.length === 0 ? (
                            <tr><td colSpan="4" className="px-6 py-12 text-center text-gray-400">Veri bekleniyor...</td></tr>
                        ) : (
                            segments.map((seg, index) => (
                                <tr key={index} className="hover:bg-indigo-50">
                                    <td className="px-6 py-4 text-sm text-gray-500 font-mono">{formatTime(seg.start)}</td>
                                    <td className="px-6 py-4"><span className="bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded-full">{seg.speaker}</span></td>
                                    <td className="px-6 py-4 text-sm text-gray-900">{seg.text}</td>
                                    <td className="px-6 py-4 text-sm">{seg.textSentiment}</td>
                                </tr>
                            ))
                        )}
                        <div ref={tableEndRef} />
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
};

export default Home;