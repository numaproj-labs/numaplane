ARG BASE_IMAGE=scratch
####################################################################################################
# base
####################################################################################################
FROM alpine:3.17 as base
ARG ARCH
RUN apk update && apk upgrade && \
    apk add ca-certificates && \
    apk --no-cache add tzdata

COPY dist/numaplane /bin/numaplane

RUN chmod +x /bin/numaplane

####################################################################################################
# numaplane
####################################################################################################
ARG BASE_IMAGE
FROM ${BASE_IMAGE} as numaplane
COPY --from=base /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=base /bin/numaplane /bin/numaplane
ENTRYPOINT [ "/bin/numaplane" ]
