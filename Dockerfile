FROM golang:1.17-alpine as build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /k8s-device-plugin-demo

FROM alpine:3.10
COPY --from=build /k8s-device-plugin-demo /k8s-device-plugin-demo
CMD [ "/k8s-device-plugin-demo" ]
