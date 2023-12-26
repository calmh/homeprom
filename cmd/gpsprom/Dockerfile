FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY bin/gpsprom-linux-${TARGETARCH} /bin/gpsprom
ENTRYPOINT ["/bin/gpsprom"]
