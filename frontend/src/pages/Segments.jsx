import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Loader } from 'lucide-react';

const Segments = () => {
    const { id } = useParams();
    const navigate = useNavigate();
    const [segments, setSegments] = useState([]);
    const [loading, setLoading] = useState(true);

    // Backend'den detayları çek
    useEffect(() => {
        if (id) {
            // Go API Endpoint: /api/segments?id=...
            fetch(`/api/segments?id=${id}`)
                .then((res) => res.json())
                .then((data) => {
                    setSegments(data || []);
                    setLoading(false);
                })
                .catch((err) => {
                    console.error('Segmentler çekilemedi:', err);
                    setLoading(false);
                });
        }
    }, [id]);

    // Saniyeyi (float) dakika:saniye formatına çevirir (Örn: 65.5 -> 01:05)
    const formatTime = (seconds) => {
        if (!seconds && seconds !== 0) return '00:00';
        const totalSeconds = Math.floor(seconds);
        const mins = Math.floor(totalSeconds / 60);
        const secs = totalSeconds % 60;
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    };

    return (
        <div className="p-6">
            <button
                onClick={() => navigate(-1)}
                className="flex items-center text-gray-600 mb-4 hover:text-indigo-600 transition-colors"
            >
                <ArrowLeft size={20} className="mr-2" /> Kayıtlara Dön
            </button>

            <h1 className="text-2xl font-bold text-gray-800 mb-2">Kayıt Detayı <span className="text-indigo-600 text-lg font-mono">#{id}</span></h1>
            <p className="text-gray-500 mb-6">Konuşma segmentleri ve detaylı duygu analizi.</p>

            {loading ? (
                <div className="flex justify-center items-center h-32 text-indigo-600">
                    <Loader className="animate-spin mr-2" /> Analiz verileri yükleniyor...
                </div>
            ) : (
                <div className="bg-white rounded-lg shadow overflow-hidden">
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Zaman Aralığı</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konuşmacı</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Metin</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duygu (Metin)</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duygu (Ses)</th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {segments.length === 0 ? (
                            <tr>
                                <td colSpan="5" className="px-6 py-8 text-center text-gray-500">
                                    Bu kayıt için detaylı segment verisi bulunamadı.
                                </td>
                            </tr>
                        ) : (
                            segments.map((seg, index) => (
                                <tr key={index} className="hover:bg-indigo-50 transition-colors">
                                    <td className="px-6 py-4 whitespace-nowrap font-mono text-xs text-gray-500">
                                        {formatTime(seg.start)} - {formatTime(seg.end)}
                                    </td>
                                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          seg.speaker === 'Bilinmiyor' ? 'bg-gray-100 text-gray-800' : 'bg-indigo-100 text-indigo-800'
                      }`}>
                        {seg.speaker || 'Bilinmiyor'}
                      </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-800 max-w-lg">
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
                                    <td className="px-6 py-4 whitespace-nowrap text-sm text-purple-700 font-medium">
                                        {seg.voiceSentiment || '-'}
                                    </td>
                                </tr>
                            ))
                        )}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default Segments;