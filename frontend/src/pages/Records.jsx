import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader } from 'lucide-react';

const Records = () => {
    const navigate = useNavigate();
    const [records, setRecords] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetch('/api/records')
            .then((res) => res.json())
            .then((data) => {
                setRecords(data || []);
                setLoading(false);
            })
            .catch((err) => {
                console.error('Kayıtlar çekilemedi:', err);
                setLoading(false);
            });
    }, []);

    if (loading) {
        return <div className="flex justify-center items-center h-64 text-indigo-600"><Loader className="animate-spin mr-2" /> Yükleniyor...</div>;
    }

    return (
        <div className="p-6">
            <h1 className="text-2xl font-bold text-gray-800 mb-6">Analiz Edilen Kayıtlar</h1>
            <div className="bg-white rounded-lg shadow overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                    <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Tarih</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Süre</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konu</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konuşmacılar</th>
                    </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                    {records.length === 0 ? (
                        <tr><td colSpan="4" className="px-6 py-8 text-center text-gray-500">Henüz kayıt bulunmamaktadır.</td></tr>
                    ) : (
                        records.map((record) => (
                            <tr key={record.id} onClick={() => navigate(`/records/${record.id}`)} className="cursor-pointer hover:bg-indigo-50 transition-colors">
                                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-700">{record.date}</td>
                                <td className="px-6 py-4 whitespace-nowrap font-mono text-sm text-gray-600">{record.duration}</td>
                                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-800">{record.topic || 'Genel'}</td>
                                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                                    {record.speakers && record.speakers.length > 0 ? record.speakers.join(', ') : 'Bilinmiyor'}
                                </td>
                            </tr>
                        ))
                    )}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default Records;