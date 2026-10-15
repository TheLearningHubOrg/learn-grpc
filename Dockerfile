# Production Dockerfile for gRPC Microservice
FROM node:20-alpine AS runner

WORKDIR /app
COPY package.json ./
RUN npm install

COPY . .

EXPOSE 50051

CMD ["npm", "start"]
