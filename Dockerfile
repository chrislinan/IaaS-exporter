FROM dockerio.int.repositories.cloud.sap/library/golang:1.19-alpine as build

RUN apk upgrade --no-cache --force
RUN apk add --update build-base make git

ENV GOPROXY=goproxy.cn

WORKDIR /go/src/github.wdf.sap.corp/DBaaS/iaas-exporter

# Compile
COPY ./ /go/src/github.wdf.sap.corp/DBaaS/iaas-exporter
RUN make dependencies
RUN make build
#############################################
# FINAL IMAGE
#############################################
FROM gcr-io.int.repositories.cloud.sap/distroless/static
COPY --from=build /go/src/github.wdf.sap.corp/DBaaS/iaas-exporter/iaas-exporter /
USER 1000:1000
ENTRYPOINT ["/iaas-exporter"]
