import React, { useState, useEffect, useRef } from 'react';
import { UserPlus, Mic, Save, StopCircle, Loader } from 'lucide-react';

const Users = () => {
    const [users, setUsers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);

    // Form ve Kayıt State'leri
    const [formData, setFormData] = useState({ name: '', surname: '' });
    const [isRecording, setIsRecording] = useState(false);
    const [audioBlob, setAudioBlob] = useState(null);

    const mediaRecorderRef = useRef(null);
    const audioChunksRef = useRef([]);

    // 1. Kullanıcı Listesini Çek (GET)
    const fetchUsers = async () => {
        try {
            const res = await fetch('/api/users');
            const data = await res.json();
            setUsers(data || []);
        } catch (error) {
            console.error("Kullanıcılar çekilemedi:", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchUsers();
    }, []);

    // --- Basitleştirilmiş Ses Kayıt İşlemleri ---
    const startRecording = async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            // Tarayıcının desteklediği varsayılan formatta kayıt (Genelde WebM)
            mediaRecorderRef.current = new MediaRecorder(stream);
            audioChunksRef.current = [];

            mediaRecorderRef.current.ondataavailable = (event) => {
                if (event.data.size > 0) audioChunksRef.current.push(event.data);
            };

            mediaRecorderRef.current.onstop = () => {
                // Ham veriyi Blob haline getir (Backend bunu WAV'a çevirecek)
                const blob = new Blob(audioChunksRef.current, { type: 'audio/webm' });
                setAudioBlob(blob);
            };

            mediaRecorderRef.current.start();
            setIsRecording(true);
        } catch (error) {
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

    // 2. Yeni Kullanıcı Kaydet (POST - FormData)
    const handleSaveUser = async () => {
        if (!formData.name || !formData.surname) return alert("İsim/Soyisim giriniz.");
        if (!audioBlob) return alert("Ses kaydı yapınız.");

        const data = new FormData();
        data.append('name', formData.name);
        data.append('surname', formData.surname);
        // Dosyayı 'recording.webm' olarak gönderiyoruz, backend uzantıyı kontrol edip dönüştürmeli
        data.append('voice_record_file', audioBlob, 'recording.webm');

        try {
            const res = await fetch('/api/record_user', {
                method: 'POST',
                body: data
            });

            if (res.ok) {
                alert("Kullanıcı başarıyla oluşturuldu.");
                setShowModal(false);
                setFormData({ name: '', surname: '' });
                setAudioBlob(null);
                fetchUsers(); // Listeyi yenile
            } else {
                alert("Kaydetme başarısız oldu.");
            }
        } catch (error) {
            console.error("Hata:", error);
            alert("Sunucu hatası.");
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
                {loading ? (
                    <div className="p-8 text-center text-gray-500 flex justify-center items-center"><Loader className="animate-spin mr-2"/> Yükleniyor...</div>
                ) : (
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">İsim</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Soyisim</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Kayıt Tarihi</th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {users.map((user, idx) => (
                            <tr key={user.id || idx}>
                                <td className="px-6 py-4">{user.name}</td>
                                <td className="px-6 py-4">{user.surname}</td>
                                <td className="px-6 py-4 text-sm text-gray-500">{user.date || '-'}</td>
                            </tr>
                        ))}
                        </tbody>
                    </table>
                )}
            </div>

            {showModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
                    <div className="bg-white rounded-xl max-w-md w-full p-6">
                        <h2 className="text-xl font-bold mb-4">Yeni Kişi Kaydı</h2>
                        <div className="space-y-4">
                            <input placeholder="İsim" className="w-full border p-2 rounded" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
                            <input placeholder="Soyisim" className="w-full border p-2 rounded" value={formData.surname} onChange={e => setFormData({...formData, surname: e.target.value})} />

                            <div className="bg-blue-50 p-4 rounded text-sm text-blue-800">
                                <p className="mb-2 font-semibold">Ses Kaydı:</p>
                                Lütfen isminizi ve soyisminizi net bir şekilde söyleyiniz.
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