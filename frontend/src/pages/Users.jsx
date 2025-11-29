import React, { useState, useEffect, useRef } from 'react';
import { useWebSocket } from '../context/WebSocketContext';
import { UserPlus, Mic, Save, StopCircle, Play } from 'lucide-react';

const Users = () => {
    const { sendMessage, lastMessage } = useWebSocket();
    const [users, setUsers] = useState([]);
    const [showModal, setShowModal] = useState(false);
    const [formData, setFormData] = useState({ name: '', surname: '' });

    const [isRecording, setIsRecording] = useState(false);
    const [audioBlob, setAudioBlob] = useState(null);
    const mediaRecorderRef = useRef(null);
    const audioChunksRef = useRef([]);

    // --- WAV DÖNÜŞTÜRÜCÜ YARDIMCI FONKSİYONLAR ---
    const writeString = (view, offset, string) => {
        for (let i = 0; i < string.length; i++) {
            view.setUint8(offset + i, string.charCodeAt(i));
        }
    };

    const floatTo16BitPCM = (output, offset, input) => {
        for (let i = 0; i < input.length; i++, offset += 2) {
            let s = Math.max(-1, Math.min(1, input[i]));
            s = s < 0 ? s * 0x8000 : s * 0x7FFF;
            output.setInt16(offset, s, true);
        }
    };

    const exportWAV = (audioBuffer) => {
        const format = 1; // PCM
        const numChannels = 1; // Mono
        const sampleRate = audioBuffer.sampleRate;
        const bitDepth = 16;

        // Sadece ilk kanalı al (Mono)
        const channelData = audioBuffer.getChannelData(0);
        const bytesPerSample = bitDepth / 8;
        const blockAlign = numChannels * bytesPerSample;

        const buffer = new ArrayBuffer(44 + channelData.length * bytesPerSample);
        const view = new DataView(buffer);

        /* RIFF identifier */
        writeString(view, 0, 'RIFF');
        /* RIFF chunk length */
        view.setUint32(4, 36 + channelData.length * bytesPerSample, true);
        /* RIFF type */
        writeString(view, 8, 'WAVE');
        /* format chunk identifier */
        writeString(view, 12, 'fmt ');
        /* format chunk length */
        view.setUint32(16, 16, true);
        /* sample format (raw) */
        view.setUint16(20, format, true);
        /* channel count */
        view.setUint16(22, numChannels, true);
        /* sample rate */
        view.setUint32(24, sampleRate, true);
        /* byte rate (sample rate * block align) */
        view.setUint32(28, sampleRate * blockAlign, true);
        /* block align (channel count * bytes per sample) */
        view.setUint16(32, blockAlign, true);
        /* bits per sample */
        view.setUint16(34, bitDepth, true);
        /* data chunk identifier */
        writeString(view, 36, 'data');
        /* data chunk length */
        view.setUint32(40, channelData.length * bytesPerSample, true);

        floatTo16BitPCM(view, 44, channelData);

        return new Blob([view], { type: 'audio/wav' });
    };
    // ------------------------------------------------

    useEffect(() => {
        // Sayfa açılışında listeyi çek
        if (sendMessage) {
            sendMessage(JSON.stringify({ type: 'get_users' }));
        }
    }, [sendMessage]);

    useEffect(() => {
        if (lastMessage) {
            try {
                let msg;
                if (lastMessage.data) {
                    msg = JSON.parse(lastMessage.data);
                } else if (typeof lastMessage === 'string') {
                    msg = JSON.parse(lastMessage);
                } else {
                    msg = lastMessage;
                }

                if (msg.type === 'users_list') {
                    setUsers(msg.payload);
                } else if (msg.type === 'notification') {
                    alert(msg.payload);
                    setShowModal(false);
                    setFormData({ name: '', surname: '' });
                    setAudioBlob(null);
                    sendMessage(JSON.stringify({ type: 'get_users' }));
                } else if (msg.type === 'error') {
                    alert("Hata: " + msg.payload);
                }
            } catch (error) {
                console.error("WS Message Error:", error);
            }
        }
    }, [lastMessage, sendMessage]);

    const startRecording = async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            mediaRecorderRef.current = new MediaRecorder(stream);
            audioChunksRef.current = [];

            mediaRecorderRef.current.ondataavailable = (event) => {
                if (event.data.size > 0) {
                    audioChunksRef.current.push(event.data);
                }
            };

            mediaRecorderRef.current.onstop = async () => {
                // 1. WebM Blob oluştur
                const webmBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' });

                // 2. WebM'i AudioContext ile çözüp WAV'a çevir
                const arrayBuffer = await webmBlob.arrayBuffer();
                const audioContext = new (window.AudioContext || window.webkitAudioContext)();
                const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);

                // 3. WAV Blob oluştur
                const wavBlob = exportWAV(audioBuffer);
                setAudioBlob(wavBlob);
            };

            mediaRecorderRef.current.start();
            setIsRecording(true);
        } catch (error) {
            console.error("Mikrofon hatası:", error);
            alert("Mikrofona erişilemedi.");
        }
    };

    const stopRecording = () => {
        if (mediaRecorderRef.current && isRecording) {
            mediaRecorderRef.current.stop();
            setIsRecording(false);
            mediaRecorderRef.current.stream.getTracks().forEach(track => track.stop());
        }
    };

    const blobToBase64 = (blob) => {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onloadend = () => {
                const base64String = reader.result.split(',')[1];
                resolve(base64String);
            };
            reader.onerror = reject;
            reader.readAsDataURL(blob);
        });
    };

    const handleSaveUser = async () => {
        if (!formData.name || !formData.surname) return alert("İsim/Soyisim giriniz.");
        if (!audioBlob) return alert("Ses kaydı yapınız.");

        try {
            const audioBase64 = await blobToBase64(audioBlob);
            const payload = {
                type: 'create_user',
                data: {
                    name: formData.name,
                    surname: formData.surname,
                    audio_base64: audioBase64
                }
            };
            sendMessage(JSON.stringify(payload));
        } catch (error) {
            console.error("Hata:", error);
            alert("Dosya hazırlanamadı.");
        }
    };

    return (
        <div className="p-6">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold text-gray-800">Kayıtlı Kişiler</h1>
                <button onClick={() => setShowModal(true)} className="bg-indigo-600 text-white px-4 py-2 rounded-lg flex items-center gap-2 hover:bg-indigo-700">
                    <UserPlus size={20} /> Yeni Kullanıcı Ekle
                </button>
            </div>

            <div className="bg-white rounded-lg shadow overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                    <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">İsim</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Soyisim</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Tarih</th>
                    </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                    {users?.map((user) => (
                        <tr key={user.id}>
                            <td className="px-6 py-4">{user.name}</td>
                            <td className="px-6 py-4">{user.surname}</td>
                            <td className="px-6 py-4 text-sm text-gray-500">{user.date}</td>
                        </tr>
                    ))}
                    </tbody>
                </table>
            </div>

            {showModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
                    <div className="bg-white rounded-xl max-w-md w-full p-6">
                        <h2 className="text-xl font-bold mb-4">Yeni Kişi Kaydı</h2>
                        <div className="space-y-4">
                            <input placeholder="İsim" className="w-full border p-2 rounded" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
                            <input placeholder="Soyisim" className="w-full border p-2 rounded" value={formData.surname} onChange={e => setFormData({...formData, surname: e.target.value})} />

                            <div className="bg-blue-50 p-4 rounded text-sm text-blue-800 max-h-40 overflow-y-auto">
                                <p className="mb-2 font-semibold">Lütfen Okuyunuz:</p>
                                "Günümüzde yapay zeka teknolojileri, hayatımızın her alanına hızla entegre olmaya devam ediyor..."
                            </div>

                            {!isRecording ? (
                                <button onClick={startRecording} className={`w-full py-3 rounded flex items-center justify-center gap-2 ${audioBlob ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-700'}`}>
                                    <Mic size={20} /> {audioBlob ? 'Yeniden Kaydet' : 'Ses Kaydet'}
                                </button>
                            ) : (
                                <button onClick={stopRecording} className="w-full py-3 rounded flex items-center justify-center gap-2 bg-red-500 text-white animate-pulse">
                                    <StopCircle size={20} /> Kaydı Bitir
                                </button>
                            )}

                            <div className="flex gap-2 mt-4 pt-4 border-t">
                                <button onClick={() => setShowModal(false)} className="flex-1 border py-2 rounded">İptal</button>
                                <button onClick={handleSaveUser} disabled={!audioBlob || isRecording} className={`flex-1 text-white py-2 rounded flex justify-center gap-2 ${(!audioBlob || isRecording) ? 'bg-gray-400' : 'bg-indigo-600'}`}>
                                    <Save size={18}/> Kaydet
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Users;