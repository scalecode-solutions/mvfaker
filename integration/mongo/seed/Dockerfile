# Build mvfaker, then bake it into a mongo image (which ships mongoimport +
# mongosh) so the seeder can generate NDJSON and load it.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mvfaker ./cmd/mvfaker
RUN CGO_ENABLED=0 go build -o /embedded ./examples/embedded

FROM mongo:7
COPY --from=build /mvfaker /usr/local/bin/mvfaker
COPY --from=build /embedded /usr/local/bin/embedded
COPY integration/mongo/seed/forum.hcl /forum.hcl
COPY integration/mongo/seed/load.sh /usr/local/bin/load.sh
RUN chmod +x /usr/local/bin/load.sh
ENTRYPOINT ["/usr/local/bin/load.sh"]
