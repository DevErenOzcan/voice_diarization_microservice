# frontend/Dockerfile
FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

# Frontend 3000 portunda çalışıyor
EXPOSE 3000

# Vite'ı dışarıdan erişilebilir (0.0.0.0) yapmak için host bayrağı kullanıyoruz
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]