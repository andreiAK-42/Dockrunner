FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# URL бэкенда вшивается на этапе сборки (браузер ходит в API напрямую).
# Пишем его в .env.production, чтобы Vite гарантированно подхватил значение
# именно из build-arg, а не из process.env.
ARG VITE_API_URL=http://localhost:8080
ARG VITE_VICTORIA_LOGS_URL=http://localhost:9428
ARG VITE_VICTORIA_METRICS_URL=http://localhost:8428
RUN printf 'VITE_API_URL=%s\nVITE_VICTORIA_LOGS_URL=%s\nVITE_VICTORIA_METRICS_URL=%s\n' \
  "$VITE_API_URL" "$VITE_VICTORIA_LOGS_URL" "$VITE_VICTORIA_METRICS_URL" > .env.production
RUN npm run build

FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80

