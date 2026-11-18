FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/api

FROM alpine:3.22
RUN adduser -D -u 10001 dtr && mkdir -p /data/photos && chown -R dtr:dtr /data
COPY --from=build /out/api /app/api
USER dtr
ENV PHOTO_DIR=/data/photos ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
