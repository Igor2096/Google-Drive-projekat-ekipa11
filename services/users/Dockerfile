FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o users .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/users .
EXPOSE 8081
CMD ["./users"]
