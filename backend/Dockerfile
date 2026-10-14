FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/campusclaw-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/campusclaw-api /app/campusclaw-api
RUN mkdir -p /data/uploads && chown -R app:app /data/uploads
USER app
EXPOSE 8080
ENTRYPOINT ["/app/campusclaw-api"]
