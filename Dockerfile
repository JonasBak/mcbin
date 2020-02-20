FROM golang:alpine as build

WORKDIR /build

RUN apk update && apk add ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . /build

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /build/main .

CMD ["./main", "serve"]
