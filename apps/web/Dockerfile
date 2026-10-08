# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /app
# pnpm theo "packageManager" trong package.json (corepack tải đúng bản).
RUN corepack enable
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm exec vite build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
