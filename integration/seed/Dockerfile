# Build context is the mvfaker repo root, so we can build the mvfaker binary,
# then bake it into a postgres-client image that loads the generated seed.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mvfaker ./cmd/mvfaker

FROM postgres:16-alpine
COPY --from=build /mvfaker /usr/local/bin/mvfaker
COPY integration/seed/forum.hcl /forum.hcl
COPY integration/seed/load.sh /usr/local/bin/load.sh
RUN chmod +x /usr/local/bin/load.sh
ENTRYPOINT ["/usr/local/bin/load.sh"]
