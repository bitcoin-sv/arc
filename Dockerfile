FROM ubuntu:latest

RUN apt-get update && apt-get install ca-certificates -y && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY ./build/arc_linux_amd64 /service/arc

WORKDIR /service

RUN chmod +x arc

EXPOSE 9090

CMD ["/service/arc"]