FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY bin/upsprom-linux-${TARGETARCH} /bin/upsprom
ENTRYPOINT ["/bin/upsprom"]
