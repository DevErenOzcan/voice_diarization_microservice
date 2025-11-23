import React from 'react';
import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom';
import { WebSocketProvider, useWebSocket } from './context/WebSocketContext';
import Home from './pages/Home';
import Users from './pages/Users';
import Records from './pages/Records';
import Segments from './pages/Segments';
import { Mic, Users as UsersIcon, FileText } from 'lucide-react';

// Navigasyon bileşeni
const NavBar = () => {
    const location = useLocation();
    const { isConnected } = useWebSocket();

    const navClass = (path) =>
        `flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
            location.pathname === path ? 'bg-indigo-700 text-white' : 'text-indigo-100 hover:bg-indigo-600'
        }`;

    return (
        <nav className="bg-indigo-800 text-white p-4 shadow-lg">
            <div className="container mx-auto flex justify-between items-center">
                <div className="text-xl font-bold flex items-center gap-2">
                    AI Sekreter
                    <span className={`text-xs px-2 py-0.5 rounded-full ${isConnected ? 'bg-green-400 text-green-900' : 'bg-red-400 text-red-900'}`}>
            {isConnected ? 'Online' : 'Offline'}
          </span>
                </div>
                <div className="flex gap-4">
                    <Link to="/" className={navClass('/')}><Mic size={18}/> Canlı Analiz</Link>
                    <Link to="/users" className={navClass('/users')}><UsersIcon size={18}/> Kişiler</Link>
                    <Link to="/records" className={navClass('/records')}><FileText size={18}/> Kayıtlar</Link>
                </div>
            </div>
        </nav>
    );
};

function App() {
    return (
        <WebSocketProvider>
            <Router>
                <div className="min-h-screen bg-gray-100">
                    <NavBar />
                    <main className="container mx-auto mt-6">
                        <Routes>
                            <Route path="/" element={<Home />} />
                            <Route path="/users" element={<Users />} />
                            <Route path="/records" element={<Records />} />
                            <Route path="/records/:id" element={<Segments />} />
                        </Routes>
                    </main>
                </div>
            </Router>
        </WebSocketProvider>
    );
}

export default App;