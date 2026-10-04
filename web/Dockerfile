# Dev: hot reload, source bind-mounted (see docker-compose.yml).
FROM node:26-alpine AS dev
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
CMD ["npx", "vite", "--host", "0.0.0.0"]

FROM dev AS builder
RUN npx vite build

# Production: static files only, non-root. Needs an auth-adding /api proxy in front.
FROM nginxinc/nginx-unprivileged:alpine AS production
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 8080
