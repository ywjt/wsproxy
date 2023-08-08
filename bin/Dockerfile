FROM busybox
WORKDIR /
COPY wsproxy /

EXPOSE 1443

LABEL org.opencontainers.image.authors="SunshineKoo"
LABEL org.opencontainers.image.version="2.3.1-beta"

ENTRYPOINT ["./wsproxy"]
CMD ["-h"]
