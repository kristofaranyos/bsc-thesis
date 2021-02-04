FROM golang:alpine AS build
WORKDIR /src/
COPY go/* /src/
RUN CGO_ENABLED=0 go build -o /bin/goservice

FROM scratch
COPY --from=build /bin/goservice /bin/goservice
EXPOSE 8080
ENTRYPOINT ["/bin/goservice"]
