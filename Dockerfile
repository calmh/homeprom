FROM golang AS builder

WORKDIR /src
COPY . .
ENV CGO_ENABLED=0
RUN go build -v ./cmd/hanprom

FROM alpine

EXPOSE 2115/tcp

COPY --from=builder /src/hanprom /bin/hanprom

ENTRYPOINT ["/bin/hanprom"]
