FROM golang:1.27.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /trip-service ./cmd/trip-service


FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /trip-service /trip-service

EXPOSE 8080

ENTRYPOINT ["/trip-service"]