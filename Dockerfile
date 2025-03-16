#Step 1
FROM golang:1.23-alpine as builder

WORKDIR /app
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -o velero-notifications .

#Step 2
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/velero-notifications /velero-notifications
COPY --from=builder /app/config/config.yaml /config/config.yaml

ENTRYPOINT ["./velero-notifications"]