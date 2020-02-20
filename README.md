# mcbin
> simple pasteboard that puts objects in minio/s3

## Development
Start and initialize minio with `docker-compose up -d` and set test environment with `eval $(cat .env.test)`

Start the server with `go run . serve`

Test it out with `curl --data-binary @main.go localhost:3000`