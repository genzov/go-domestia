FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETARCH TARGETVARIANT
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -o /out/go-domestia

FROM scratch
ENV CONFIG_PATH=/data/options.json
COPY --from=builder /out/go-domestia /go-domestia

ENTRYPOINT ["/go-domestia"]
