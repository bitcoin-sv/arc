FROM ubuntu:latest

RUN apt-get update && apt-get install ca-certificates wget -y && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY ./build/arc_linux_amd64 /service/arc

WORKDIR /service

# Add grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.24 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

RUN chmod +x arc

EXPOSE 9090

CMD ["/service/arc"]
