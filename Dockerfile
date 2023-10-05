FROM golang:1.21.1

COPY ./build/arc_linux_amd64 /service/arc

WORKDIR /service

RUN chmod +x arc

EXPOSE 9090

CMD ["/service/arc"]
