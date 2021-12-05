FROM golang:1.16 as modules
ADD go.mod go.sum /m/
RUN cd /m && go mod download

FROM golang:1.16 as builder
COPY --from=modules /go/pkg /go/pkg
RUN mkdir -p /src
ADD . /src
WORKDIR /src
RUN useradd -u 10001 myapp
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /myapp ./

FROM busybox
COPY --from=builder /etc/passwd /etc/passwd
USER myapp
COPY --from=builder /myapp /myapp
EXPOSE 9090
CMD ["/myapp"]


# docker build -t docker.io/pehks1980/be2hw4 .
# docker run -i -p=9090:9090 pehks1980/be2hw4