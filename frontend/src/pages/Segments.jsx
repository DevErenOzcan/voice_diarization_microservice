import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Loader } from 'lucide-react';

const Segments = () => {
    const { id } = useParams();
    const navigate = useNavigate();
    const [segments, setSegments] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (id) {
            // REST Standartlarına daha uygun: /api/records/123 gibi düşünülebilir
            // Ama mevcut isteğiniz "/api/records/:id" route'u olduğu için backend muhtemelen
            // query param veya path param bekliyor.
            fetch(`/api/records/${id}`)
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

    const formatTime = (seconds) => {
        if (!seconds && seconds !== 0) return '00:00';
        const totalSeconds = Math.floor(seconds);
        const mins = Math.floor(totalSeconds / 60);
        const secs = totalSeconds % 60;
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    };

    return (
        <div className="p-6">
            <button onClick={() => navigate(-1)} className="flex items-center text-gray-600 mb-4 hover:text-indigo-600 transition-colors">
                <ArrowLeft size={20} className="mr-2" /> Kayıtlara Dön
            </button>

            <h1 className="text-2xl font-bold text-gray-800 mb-2">Kayıt Detayı <span className="text-indigo-600 text-lg font-mono">#{id}</span></h1>

            {loading ? (
                <div className="flex justify-center items-center h-32 text-indigo-600"><Loader className="animate-spin mr-2" /> Analiz verileri yükleniyor...</div>
            ) : (
                <div className="bg-white rounded-lg shadow overflow-hidden">
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Zaman</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Konuşmacı</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Metin</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duygu (Metin)</th>
                        </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                        {segments.length === 0 ? (
                            <tr><td colSpan="4" className="px-6 py-8 text-center text-gray-500">Segment bulunamadı.</td></tr>
                        ) : (
                            segments.map((seg, index) => (
                                <tr key={index} className="hover:bg-indigo-50 transition-colors">
                                    <td className="px-6 py-4 whitespace-nowrap font-mono text-xs text-gray-500">{formatTime(seg.start)} - {formatTime(seg.end)}</td>
                                    <td className="px-6 py-4 whitespace-nowrap">
                                        <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${seg.speaker === 'Bilinmiyor' ? 'bg-gray-100 text-gray-800' : 'bg-indigo-100 text-indigo-800'}`}>
                                            {seg.speaker || 'Bilinmiyor'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-800 max-w-lg">{seg.text}</td>
                                    <td className="px-6 py-4 whitespace-nowrap text-sm">{seg.textSentiment || '-'}</td>
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