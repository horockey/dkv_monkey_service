FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o /build/dkv_monkey_service ./cmd/dkv_monkey_service

FROM alpine
COPY --from=builder /build/dkv_monkey_service /usr/bin/dkv_monkey_service
ENV TZ=Europe/Moscow
CMD [ "dkv_monkey_service" ]