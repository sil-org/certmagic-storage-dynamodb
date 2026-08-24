FROM golang:1.26

COPY . /certmagic-storage-dynamodb

WORKDIR /certmagic-storage-dynamodb

RUN go get ./...

CMD ["bash"]
