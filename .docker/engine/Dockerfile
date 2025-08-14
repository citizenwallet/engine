FROM golang:1.23.1-alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm64 \
    GOPROXY=https://goproxy.cn,direct 
    # 'cn' is used because errors appears to be TLS handshake issues specifically on Hetzner servers.

# create build folder
RUN mkdir -p /engine-build

# create binaries folder
RUN mkdir -p /engine

# move into build folder
WORKDIR /engine-build

# copy all go files into our container
COPY . .

# copy the .env file into our binaries folder
COPY .env /engine/.env
# COPY firebase.json /cw/firebase.json

# install all plugins
RUN go mod download

# build
RUN go build -o /engine/main ./cmd/main.go

# clean up container
RUN rm -rf /engine-build

# Build a small image
FROM golang:1.23.1-alpine

RUN mkdir -p /engine

COPY --from=builder /engine/main /engine

COPY --from=builder /engine/.env /engine
# COPY --from=builder /cw/firebase.json /cw

# define the command to be run on launch
ENTRYPOINT ["/engine/main"]

CMD ["-env", "/engine/.env", "-notify"]