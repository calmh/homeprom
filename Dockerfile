FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY hanprom-linux-${TARGETARCH} /bin/hanprom
ENTRYPOINT ["/bin/hanprom"]
