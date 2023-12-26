FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY bin/hanprom-linux-${TARGETARCH} /bin/hanprom
ENTRYPOINT ["/bin/hanprom"]
