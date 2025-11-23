import React, { createContext, useRef, useEffect, useState, useContext } from 'react';

const WebSocketContext = createContext(null);

export const WebSocketProvider = ({ children }) => {
    const socketRef = useRef(null);
    const [isConnected, setIsConnected] = useState(false);
    const [lastMessage, setLastMessage] = useState(null);

    // .env dosyasından URL'i alıyoruz
    // Eğer .env okunamazsa güvenlik önlemi olarak varsayılan localhost'a düşebiliriz
    const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws';

    useEffect(() => {
        const connect = () => {
            console.log(`Bağlanılıyor: ${WS_URL}`);
            socketRef.current = new WebSocket(WS_URL); // Artık hardcoded değil

            socketRef.current.onopen = () => {
                console.log('Gateway bağlantısı sağlandı.');
                setIsConnected(true);
            };

            socketRef.current.onmessage = (event) => {
                try {
                    // Backend'den gelen JSON verilerini parse ediyoruz
                    const data = JSON.parse(event.data);
                    setLastMessage(data);
                } catch (e) {
                    console.log('Binary veya Text olmayan veri alındı:', event.data);
                }
            };

            socketRef.current.onclose = () => {
                console.log('Bağlantı koptu, tekrar deneniyor...');
                setIsConnected(false);
                setTimeout(connect, 3000); // 3 saniye sonra tekrar dene
            };
        };

        connect();

        return () => {
            if (socketRef.current) socketRef.current.close();
        };
    }, []);

    // Veri gönderme fonksiyonu (Binary ses veya JSON komut)
    const sendMessage = (data) => {
        if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
            socketRef.current.send(data);
        } else {
            console.warn('Socket bağlı değil, veri gönderilemedi.');
        }
    };

    return (
        <WebSocketContext.Provider value={{ socketRef, isConnected, lastMessage, sendMessage }}>
            {children}
        </WebSocketContext.Provider>
    );
};

export const useWebSocket = () => useContext(WebSocketContext);