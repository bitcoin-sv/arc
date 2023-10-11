FROM ubuntu:latest

COPY ./build/arc_linux_amd64 /service/arc

WORKDIR /service

RUN chmod +x arc

EXPOSE 9090

CMD ["/service/arc"]
