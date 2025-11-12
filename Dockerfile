FROM golang:1.25

COPY . /certmagic-storage-dynamodb

WORKDIR /certmagic-storage-dynamodb

RUN go get ./...

CMD ["bash"]
