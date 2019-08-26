FROM golang as build

WORKDIR /app
ADD . /app
RUN cd /app && \
  make build-linux64

FROM alpine

COPY --from=build /app/bin/lumberman /

EXPOSE 9090

ENTRYPOINT ["/lumberman"]
