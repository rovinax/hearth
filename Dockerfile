FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hearth ./cmd/hearth

FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg vips-tools
COPY --from=build /out/hearth /usr/local/bin/hearth
WORKDIR /data
ENV HEARTH_DATA_DIR=/data
ENV HEARTH_HTTP_ADDR=:8080
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["hearth"]
