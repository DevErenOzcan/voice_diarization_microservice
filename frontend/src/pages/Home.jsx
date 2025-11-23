import React, { useState, useRef, useEffect } from 'react';
import { useWebSocket } from '../context/WebSocketContext';
import { floatTo16BitPCM } from '../utils/audioUtils';
import { Mic, Square, Activity } from 'lucide-react';

const Home = () => {
    const { socketRef, lastMessage } = useWebSocket();
    const [isRecording, setIsRecording] = useState(false);
    const [analysis, setAnalysis] = useState({ text: '', textSentiment: '', voiceSentiment: '', speaker: '' });

    const audioContextRef = useRef(null);
    const processorRef = useRef(null);
    const inputRef = useRef(null);

    // Backend'den gelen anlık analiz sonuçlarını dinle
    useEffect(() => {
        if (lastMessage && lastMessage.type === 'live_analysis') {
            setAnalysis(lastMessage.payload);
        }
    }, [lastMessage]);

    const startRecording = async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

            // --- Senin Verdiğin Kod Başlangıcı ---
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
            // --- Senin Verdiğin Kod Bitişi ---

            setIsRecording(true);
        } catch (err) {
            console.error("Mikrofon hatası:", err);
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

    return (
        <div className="p-6 max-w-4xl mx-auto">
            <h1 className="text-2xl font-bold mb-6 text-gray-800">Canlı Ses Analizi</h1>

            <div className="bg-white rounded-xl shadow-lg p-8 mb-6 border border-gray-100">
                <div className="flex justify-center mb-8">
                    <button
                        onClick={isRecording ? stopRecording : startRecording}
                        className={`p-6 rounded-full transition-all duration-300 ${
                            isRecording ? 'bg-red-500 hover:bg-red-600 animate-pulse' : 'bg-indigo-600 hover:bg-indigo-700'
                        } text-white shadow-xl`}
                    >
                        {isRecording ? <Square size={32} /> : <Mic size={32} />}
                    </button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <h3 className="text-sm font-semibold text-gray-500 mb-2 uppercase">Algılanan Metin</h3>
                        <p className="text-gray-800 text-lg min-h-[3rem]">{analysis.text || "Konuşma bekleniyor..."}</p>
                    </div>

                    <div className="bg-gray-50 p-4 rounded-lg">
                        <h3 className="text-sm font-semibold text-gray-500 mb-2 uppercase">Analiz Sonuçları</h3>
                        <ul className="space-y-2">
                            <li className="flex items-center justify-between">
                                <span>Duygu (Metin):</span>
                                <span className="font-medium text-indigo-600">{analysis.textSentiment || "-"}</span>
                            </li>
                            <li className="flex items-center justify-between">
                                <span>Duygu (Ses):</span>
                                <span className="font-medium text-purple-600">{analysis.voiceSentiment || "-"}</span>
                            </li>
                            <li className="flex items-center justify-between">
                                <span>Konuşmacı:</span>
                                <span className="font-bold text-green-600">{analysis.speaker || "Bilinmiyor"}</span>
                            </li>
                        </ul>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Home;