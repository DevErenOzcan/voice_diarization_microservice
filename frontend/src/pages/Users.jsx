import React, { useState, useEffect, useRef } from 'react';
import { UserPlus, Mic, Save, StopCircle, Loader } from 'lucide-react';

const Users = () => {
    const [users, setUsers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);

    // Form and Recording States
    const [formData, setFormData] = useState({ name: '', surname: '' });
    const [isRecording, setIsRecording] = useState(false);
    const [audioBlob, setAudioBlob] = useState(null);

    const mediaRecorderRef = useRef(null);
    const audioChunksRef = useRef([]);

    // 1. Fetch User List (GET)
    const fetchUsers = async () => {
        try {
            const res = await fetch('/api/users');
            const data = await res.json();
            setUsers(data || []);
        } catch (error) {
            console.error("Could not fetch users:", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchUsers();
    }, []);

    // --- Simplified Voice Recording Operations ---
    const startRecording = async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            // Record in default format supported by browser (Usually WebM)
            mediaRecorderRef.current = new MediaRecorder(stream);
            audioChunksRef.current = [];

            mediaRecorderRef.current.ondataavailable = (event) => {
                if (event.data.size > 0) audioChunksRef.current.push(event.data);
            };

            mediaRecorderRef.current.onstop = () => {
                // Convert raw data to Blob (Backend should convert this to WAV)
                const blob = new Blob(audioChunksRef.current, { type: 'audio/webm' });
                setAudioBlob(blob);
            };

            mediaRecorderRef.current.start();
            setIsRecording(true);
        } catch (error) {
            alert("Microphone could not be accessed.");
        }
    };

    const stopRecording = () => {
        if (mediaRecorderRef.current && isRecording) {
            mediaRecorderRef.current.stop();
            setIsRecording(false);
            mediaRecorderRef.current.stream.getTracks().forEach(track => track.stop());
        }
    };

    // 2. Register New User (POST - FormData)
    const handleSaveUser = async () => {
        if (!formData.name || !formData.surname) return alert("Please enter Name/Surname.");
        if (!audioBlob) return alert("Please record audio.");

        const data = new FormData();
        data.append('name', formData.name);
        data.append('surname', formData.surname);
        // Sending file as 'recording.webm', backend should check extension and convert
        data.append('voice_record_file', audioBlob, 'recording.webm');

        try {
            const res = await fetch('/api/record_user', {
                method: 'POST',
                body: data
            });

            if (res.ok) {
                alert("User successfully created.");
                setShowModal(false);
                setFormData({ name: '', surname: '' });
                setAudioBlob(null);
                fetchUsers(); // Refresh list
            } else {
                alert("Save failed.");
            }
        } catch (error) {
            console.error("Error:", error);
            alert("Server error.");
        }
    };

    return (
        <div className="p-6">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold text-gray-800">Registered People</h1>
                <button onClick={() => setShowModal(true)} className="bg-indigo-600 text-white px-4 py-2 rounded-lg flex items-center gap-2 hover:bg-indigo-700">
                    <UserPlus size={20} /> Add New User
                </button>
            </div>

            <div className="bg-white rounded-lg shadow overflow-hidden">
                {loading ? (
                    <div className="p-8 text-center text-gray-500 flex justify-center items-center"><Loader className="animate-spin mr-2"/> Loading...</div>
                ) : (
                    <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Surname</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Registration Date</th>
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
                        <h2 className="text-xl font-bold mb-4">New Person Registration</h2>
                        <div className="space-y-4">
                            <input placeholder="Name" className="w-full border p-2 rounded" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
                            <input placeholder="Surname" className="w-full border p-2 rounded" value={formData.surname} onChange={e => setFormData({...formData, surname: e.target.value})} />

                            <div className="bg-blue-50 p-4 rounded text-sm text-blue-800">
                                <p className="mb-2 font-semibold">Please read the text below:</p>
                                "Life is mostly composed of moments that slip away silently while we make big plans. We cannot change yesterday; it has already happened and ended. Tomorrow is a possibility that has not yet arrived. The only real power we have is right now. Most of us spend our lives in the waiting room, waiting for the 'right time'. However, the right time is the moment you create it.

                                A minute, although it seems very short, is long enough to make a decision. You don't need hours to change the direction of your life, to take a step towards a dream you postponed, or just to say 'I can do it' to yourself. The important thing is to show the courage to take that first step. Remember, even the most magnificent peaks are climbed with small, determined steps.

                                Let today be the day you put aside excuses and embrace your potential. Take the burden of the past off your back and take a deep breath. Because the pen of your story is in your hand and you still have time to write the most beautiful chapter."
                            </div>

                            {!isRecording ? (
                                <button onClick={startRecording} className={`w-full py-3 rounded flex items-center justify-center gap-2 ${audioBlob ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-700'}`}>
                                    <Mic size={20} /> {audioBlob ? 'Re-record' : 'Record Audio'}
                                </button>
                            ) : (
                                <button onClick={stopRecording} className="w-full py-3 rounded flex items-center justify-center gap-2 bg-red-500 text-white animate-pulse">
                                    <StopCircle size={20} /> Stop Recording
                                </button>
                            )}

                            <div className="flex gap-2 mt-4 pt-4 border-t">
                                <button onClick={() => setShowModal(false)} className="flex-1 border py-2 rounded">Cancel</button>
                                <button onClick={handleSaveUser} disabled={!audioBlob || isRecording} className={`flex-1 text-white py-2 rounded flex justify-center gap-2 ${(!audioBlob || isRecording) ? 'bg-gray-400' : 'bg-indigo-600'}`}>
                                    <Save size={18}/> Save
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
