FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
# npm 11 resolves a Linux-only transitive dependency that is absent from the
# lockfile generated on Windows. `npm install` keeps the committed lockfile as
# the version baseline while completing that platform-specific entry.
RUN npm install --no-audit --no-fund

COPY public ./public
COPY src ./src
COPY tsconfig.json ./

ARG REACT_APP_API_BASE_URL=/api/v1
ENV REACT_APP_API_BASE_URL=${REACT_APP_API_BASE_URL}

RUN npm run build

FROM nginx:1.28-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/build /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://127.0.0.1/health || exit 1
