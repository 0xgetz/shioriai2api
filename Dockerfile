# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shioriai2api .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/shioriai2api /app/shioriai2api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/shioriai2api"]
