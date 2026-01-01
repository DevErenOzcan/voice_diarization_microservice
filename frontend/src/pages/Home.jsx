import React, { useState, useRef, useEffect } from 'react';
import { Mic, Square, Activity, MessageSquare } from 'lucide-react';

// audioUtils.js content moved here
const floatTo16BitPCM = (input) => {
    const output = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
        const s = Math.max(-1, Math.min(1, input[i]));
        output[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
    }
    return output;
};

const mapTextSentiment = (sentiment) => {
    const map = {
        'Olumlu': 'Positive',
        'Olumsuz': 'Negative',
        'Nötr': 'Neutral'
    };
    return map[sentiment] || sentiment;
};

const mapVoiceSentiment = (sentiment) => {
    const map = {
        'Mutlu': 'Happy',
        'Kızgın': 'Angry',
        'Üzgün': 'Sad',
        'Nötr': 'Neutral'
    };
    return map[sentiment] || sentiment;
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
                console.log('WS Connected, audio processing starting...');
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
                    console.error("JSON parse error:", e);
                }
            };

            socketRef.current.onclose = () => {
                console.log('WS Connection closed');
                setConnectionStatus('disconnected');
                setIsRecording(false);
                stopAudioProcessing();
            };

        } catch (err) {
            console.error("Initialization error:", err);
            alert("Microphone permission could not be obtained or unable to connect to server.");
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
                <h1 className="text-3xl font-bold text-gray-800 mb-2">Live Call Analysis</h1>
                <p className="text-gray-500 mb-6">
                    {connectionStatus === 'connecting' ? 'Connecting to server...' : 'Press the button below to start the call.'}
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
                        <span>Live Analysis Active</span>
                    </div>
                )}
            </div>

            <div className="flex-grow bg-white rounded-xl shadow-lg border border-gray-200 flex flex-col overflow-hidden">
                <div className="bg-gray-50 px-6 py-3 border-b border-gray-200 flex justify-between items-center">
                    <h2 className="font-bold text-gray-700 flex items-center gap-2">
                        <MessageSquare size={18}/> Segment Analysis
                    </h2>
                    <span className="text-xs bg-indigo-100 text-indigo-800 px-2 py-1 rounded-full">
                        {segments.length} Segment
                    </span>
                </div>

                <div className="overflow-y-auto flex-grow p-0">
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50 sticky top-0 z-10 shadow-sm">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-24">Duration</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-40">Speaker / Score</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase">Text</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-32">Text Sentiment</th>
                            <th className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase w-32">Voice Sentiment</th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {segments.length === 0 ? (
                            <tr><td colSpan="5" className="px-6 py-12 text-center text-gray-400">Waiting for data...</td></tr>
                        ) : (
                            segments.map((seg, index) => (
                                <tr key={index} className="hover:bg-indigo-50">
                                    <td className="px-6 py-4 text-sm text-gray-500 font-mono">{formatTime(seg.start)}</td>

                                    {/* Speaker and Score */}
                                    <td className="px-6 py-4">
                                        <div className="flex items-center gap-2">
                                            <span className="bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded-full font-bold">
                                                {seg.speaker}
                                            </span>
                                            {seg.similarity_score !== undefined && (
                                                <span className="text-xs text-gray-500 font-mono">
                                                   %{(seg.similarity_score * 100).toFixed(0)}
                                                </span>
                                            )}
                                        </div>
                                    </td>

                                    <td className="px-6 py-4 text-sm text-gray-900">{seg.text}</td>

                                    {/* Text Sentiment */}
                                    <td className="px-6 py-4">
                                        <span className={`text-xs px-2 py-1 rounded-full border ${
                                            mapTextSentiment(seg.textSentiment) === 'Positive' ? 'bg-green-50 text-green-700 border-green-200' :
                                                mapTextSentiment(seg.textSentiment) === 'Negative' ? 'bg-red-50 text-red-700 border-red-200' :
                                                    'bg-gray-100 text-gray-700 border-gray-200'
                                        }`}>
                                            {mapTextSentiment(seg.textSentiment)}
                                        </span>
                                    </td>

                                    {/* Voice Sentiment */}
                                    <td className="px-6 py-4">
                                         <span className={`text-xs px-2 py-1 rounded-full border ${
                                             mapVoiceSentiment(seg.voiceSentiment) === 'Happy' ? 'bg-green-50 text-green-700 border-green-200' :
                                                 mapVoiceSentiment(seg.voiceSentiment) === 'Angry' ? 'bg-red-50 text-red-700 border-red-200' :
                                                     mapVoiceSentiment(seg.voiceSentiment) === 'Sad' ? 'bg-orange-50 text-orange-700 border-orange-200' :
                                                         'bg-gray-100 text-gray-700 border-gray-200'
                                         }`}>
                                            {mapVoiceSentiment(seg.voiceSentiment)}
                                        </span>
                                    </td>
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
