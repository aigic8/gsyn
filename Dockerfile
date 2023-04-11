FROM golang:alpine

RUN mkdir /app

# installing protoc
RUN apk update && apk add --no-cache make protobuf-dev

# copying files and dirs
ADD ./cmd /app/cmd
ADD ./api /app/api
COPY ./go.mod /app/go.mod
COPY ./go.sum /app/go.sum

# generating protobuf files
WORKDIR /app/api/
RUN chmod +x ./generate_pb.sh
RUN ./generate_pb.sh

# building
WORKDIR /app
RUN go build /app/cmd/gsyn

# or whatever port you want
EXPOSE 1616

CMD [ "/app/gsyn", "serve" ]

