# ---- build ----
FROM golang:1.24-alpine AS build
WORKDIR /app

# Prvo samo go.mod/go.sum da bi Docker kesirao preuzete zavisnosti.
# go.sum se generise sa `go mod tidy` - zato glob, da build radi i pre toga.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /files-service ./cmd

# ---- run ----
FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
COPY --from=build /files-service /files-service
EXPOSE 8082
ENTRYPOINT ["/files-service"]
