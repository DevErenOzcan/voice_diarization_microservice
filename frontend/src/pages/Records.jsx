import React from 'react';
import { useNavigate } from 'react-router-dom';

const Records = () => {
    const navigate = useNavigate();

    // Mock Data
    const records = [
        { id: 101, duration: '00:02:15', date: '2025-11-24 10:30', topic: 'Müşteri Şikayeti', sentiment: 'Negatif', speakers: ['Ahmet Yılmaz', 'Müşteri'] },
        { id: 102, duration: '00:05:00', date: '2025-11-24 11:00', topic: 'Toplantı Notları', sentiment: 'Nötr', speakers: ['Ayşe Demir'] },
    ];

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
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duygu</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konuşmacılar</th>
                    </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                    {records.map((record) => (
                        <tr
                            key={record.id}
                            onClick={() => navigate(`/records/${record.id}`)}
                            className="cursor-pointer hover:bg-indigo-50 transition-colors"
                        >
                            <td className="px-6 py-4 whitespace-nowrap">{record.date}</td>
                            <td className="px-6 py-4 whitespace-nowrap font-mono text-sm">{record.duration}</td>
                            <td className="px-6 py-4 whitespace-nowrap">{record.topic}</td>
                            <td className="px-6 py-4 whitespace-nowrap">
                  <span className={`px-2 py-1 rounded-full text-xs ${record.sentiment === 'Negatif' ? 'bg-red-100 text-red-800' : 'bg-green-100 text-green-800'}`}>
                    {record.sentiment}
                  </span>
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">{record.speakers.join(", ")}</td>
                        </tr>
                    ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default Records;