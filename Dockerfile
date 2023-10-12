FROM golang:1.21.1

WORKDIR /service

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN go build -o ./arc main.go

EXPOSE 9090

CMD ["/service/arc"]
