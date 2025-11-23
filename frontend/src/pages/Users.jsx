import React, { useState, useEffect } from 'react';
import { useWebSocket } from '../context/WebSocketContext';
import { UserPlus, Mic, Save } from 'lucide-react';

const Users = () => {
    const { sendMessage, lastMessage } = useWebSocket();
    const [users, setUsers] = useState([]); // Bu veri normalde WS'den initial load ile gelmeli
    const [showModal, setShowModal] = useState(false);

    // Form State
    const [formData, setFormData] = useState({ name: '', surname: '' });
    const [isRecording, setIsRecording] = useState(false);

    // Mock Data (Backend bağlanınca burayı WS'den gelen veriyle doldurabilirsin)
    useEffect(() => {
        // Örnek: sendMessage({ type: 'get_users' });
        // Demo amaçlı sabit veri:
        setUsers([
            { id: 1, name: 'Ahmet', surname: 'Yılmaz', date: '2025-11-20' },
            { id: 2, name: 'Ayşe', surname: 'Demir', date: '2025-11-22' },
        ]);
    }, []);

    const handleSaveUser = () => {
        // Burada hem form verisini hem de (eğer varsa) ses blob'unu WS ile göndermelisin.
        // Basitlik adına JSON komutu gönderiyoruz:
        const payload = {
            type: 'create_user',
            data: { ...formData, date: new Date().toISOString() }
        };
        sendMessage(JSON.stringify(payload));
        setShowModal(false);
        alert("Kullanıcı kaydı backend'e iletildi.");
    };

    return (
        <div className="p-6">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold text-gray-800">Kayıtlı Kişiler</h1>
                <button
                    onClick={() => setShowModal(true)}
                    className="bg-indigo-600 text-white px-4 py-2 rounded-lg flex items-center gap-2 hover:bg-indigo-700"
                >
                    <UserPlus size={20} /> Yeni Kullanıcı Ekle
                </button>
            </div>

            {/* Tablo */}
            <div className="bg-white rounded-lg shadow overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                    <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">İsim</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Soyisim</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Kayıt Tarihi</th>
                    </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                    {users.map((user) => (
                        <tr key={user.id}>
                            <td className="px-6 py-4 whitespace-nowrap">{user.name}</td>
                            <td className="px-6 py-4 whitespace-nowrap">{user.surname}</td>
                            <td className="px-6 py-4 whitespace-nowrap">{user.date}</td>
                        </tr>
                    ))}
                    </tbody>
                </table>
            </div>

            {/* Modal */}
            {showModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4">
                    <div className="bg-white rounded-xl max-w-md w-full p-6">
                        <h2 className="text-xl font-bold mb-4">Yeni Kişi Kaydı</h2>

                        <div className="space-y-4">
                            <input
                                placeholder="İsim"
                                className="w-full border p-2 rounded"
                                onChange={e => setFormData({...formData, name: e.target.value})}
                            />
                            <input
                                placeholder="Soyisim"
                                className="w-full border p-2 rounded"
                                onChange={e => setFormData({...formData, surname: e.target.value})}
                            />

                            <div className="bg-blue-50 p-4 rounded text-sm text-blue-800">
                                Lütfen aşağıdaki metni okuyunuz:
                                <br/><strong>"Sistem güvenliği için sesimi kaydediyorum."</strong>
                            </div>

                            <button
                                className={`w-full py-3 rounded flex items-center justify-center gap-2 ${isRecording ? 'bg-red-500 text-white' : 'bg-gray-200 text-gray-700'}`}
                                onClick={() => setIsRecording(!isRecording)}
                            >
                                <Mic size={20} /> {isRecording ? 'Kaydı Durdur' : 'Ses Kaydet'}
                            </button>

                            <div className="flex gap-2 mt-4">
                                <button onClick={() => setShowModal(false)} className="flex-1 border py-2 rounded hover:bg-gray-50">İptal</button>
                                <button onClick={handleSaveUser} className="flex-1 bg-indigo-600 text-white py-2 rounded hover:bg-indigo-700 flex justify-center items-center gap-2">
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