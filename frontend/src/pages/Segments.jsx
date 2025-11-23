import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';

const Segments = () => {
    const { id } = useParams();
    const navigate = useNavigate();

    // Mock Data (Normalde ID'ye göre API'den/WS'den çekilecek)
    const segments = [
        { id: 1, start: '00:00:00', end: '00:00:15', speaker: 'Müşteri', text: 'Merhaba, ürünüm hala gelmedi.' },
        { id: 2, start: '00:00:16', end: '00:00:25', speaker: 'Ahmet Yılmaz', text: 'Merhaba efendim, hemen kontrol ediyorum.' },
    ];

    return (
        <div className="p-6">
            <button
                onClick={() => navigate(-1)}
                className="flex items-center text-gray-600 mb-4 hover:text-indigo-600"
            >
                <ArrowLeft size={20} className="mr-2" /> Kayıtlara Dön
            </button>

            <h1 className="text-2xl font-bold text-gray-800 mb-2">Kayıt Detayı #{id}</h1>
            <p className="text-gray-500 mb-6">Konuşma segmentleri ve detaylı analiz.</p>

            <div className="bg-white rounded-lg shadow overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                    <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Zaman Aralığı</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konuşmacı</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Metin</th>
                    </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                    {segments.map((seg) => (
                        <tr key={seg.id}>
                            <td className="px-6 py-4 whitespace-nowrap font-mono text-xs text-gray-500">
                                {seg.start} - {seg.end}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap font-medium text-indigo-600">{seg.speaker}</td>
                            <td className="px-6 py-4 text-gray-800">{seg.text}</td>
                        </tr>
                    ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default Segments;