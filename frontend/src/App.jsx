import React from 'react';
import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom';
import Home from './pages/Home';
import Users from './pages/Users';
import Records from './pages/Records';
import Segments from './pages/Segments';
import { Mic, Users as UsersIcon, FileText } from 'lucide-react';

const NavBar = () => {
    const location = useLocation();

    // Navbar link stili
    const navClass = (path) =>
        `flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
            location.pathname === path ? 'bg-indigo-700 text-white' : 'text-indigo-100 hover:bg-indigo-600'
        }`;

    return (
        <nav className="bg-indigo-800 text-white p-4 shadow-lg">
            <div className="container mx-auto flex justify-between items-center">
                <div className="text-xl font-bold flex items-center gap-2">
                    Voice Recognition
                </div>
                <div className="flex gap-4">
                    <Link to="/" className={navClass('/')}><Mic size={18}/> Live Analysis</Link>
                    <Link to="/users" className={navClass('/users')}><UsersIcon size={18}/> People</Link>
                    <Link to="/records" className={navClass('/records')}><FileText size={18}/> Records</Link>
                </div>
            </div>
        </nav>
    );
};

function App() {
    return (
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
    );
}

export default App;