# Golang build container
FROM golang:1.10

WORKDIR $GOPATH/src/github.com/grafana/grafana

COPY Gopkg.toml Gopkg.lock ./
COPY vendor vendor

RUN apt-get update -y
RUN apt-get install -y unzip libaio-dev libaio1

RUN rm -rf /opt/oracle/
RUN mkdir -p /opt/oracle/
COPY oracle/instantclient-basic-linux.x64-12.2.0.1.0.zip /opt/oracle/
COPY oracle/instantclient-sdk-linux.x64-12.2.0.1.0.zip /opt/oracle/

RUN unzip /opt/oracle/instantclient-basic-linux.x64-12.2.0.1.0.zip -d /opt/oracle/
RUN unzip /opt/oracle/instantclient-sdk-linux.x64-12.2.0.1.0.zip -d /opt/oracle/

RUN ln -s /opt/oracle/instantclient_12_2 /opt/oracle/instantclient
RUN ln -s /opt/oracle/instantclient/libclntsh.so.12.1 /opt/oracle/instantclient/libclntsh.so

RUN rm -rf /opt/oracle/instantclient-basic-linux.x64-12.2.0.1.0.zip
RUN rm -rf /opt/oracle/instantclient-sdk-linux.x64-12.2.0.1.0.zip

ENV LD_LIBRARY_PATH=/opt/oracle/instantclient/
ENV ORACLE_HOME=/opt/oracle/instantclient/

COPY oci8.pc /usr/lib/pkgconfig
ENV PKG_CONFIG_PATH=/usr/lib/pkgconfig

ARG DEP_ENSURE=""
RUN if [ ! -z "${DEP_ENSURE}" ]; then \
      go get -u github.com/golang/dep/cmd/dep && \
      dep ensure --vendor-only; \
    fi

COPY pkg pkg
COPY build.go build.go
COPY package.json package.json

RUN go run build.go build

# Node build container
FROM node:8

WORKDIR /usr/src/app/

COPY package.json yarn.lock ./
RUN yarn install --pure-lockfile --no-progress

COPY Gruntfile.js tsconfig.json tslint.json ./
COPY public public
COPY scripts scripts
COPY emails emails

ENV NODE_ENV production
RUN ./node_modules/.bin/grunt build

# Final container
FROM debian:stretch-slim

ARG GF_UID="472"
ARG GF_GID="472"

ENV PATH=/usr/share/grafana/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    GF_PATHS_CONFIG="/etc/grafana/grafana.ini" \
    GF_PATHS_DATA="/var/lib/grafana" \
    GF_PATHS_HOME="/usr/share/grafana" \
    GF_PATHS_LOGS="/var/log/grafana" \
    GF_PATHS_PLUGINS="/var/lib/grafana/plugins" \
    GF_PATHS_PROVISIONING="/etc/grafana/provisioning" \
    LD_LIBRARY_PATH=/opt/oracle/instantclient/ \
    ORACLE_HOME=/opt/oracle/instantclient/

WORKDIR $GF_PATHS_HOME

RUN apt-get update && apt-get install -qq -y libfontconfig ca-certificates libaio-dev libaio1 && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY conf ./conf

RUN mkdir -p "$GF_PATHS_HOME/.aws" && \
    groupadd -r -g $GF_GID grafana && \
    useradd -r -u $GF_UID -g grafana grafana && \
    mkdir -p "$GF_PATHS_PROVISIONING/datasources" \
             "$GF_PATHS_PROVISIONING/dashboards" \
             "$GF_PATHS_LOGS" \
             "$GF_PATHS_PLUGINS" \
             "$GF_PATHS_DATA" && \
    cp "$GF_PATHS_HOME/conf/sample.ini" "$GF_PATHS_CONFIG" && \
    cp "$GF_PATHS_HOME/conf/ldap.toml" /etc/grafana/ldap.toml && \
    chown -R grafana:grafana "$GF_PATHS_DATA" "$GF_PATHS_HOME/.aws" "$GF_PATHS_LOGS" "$GF_PATHS_PLUGINS" && \
    chmod 777 "$GF_PATHS_DATA" "$GF_PATHS_HOME/.aws" "$GF_PATHS_LOGS" "$GF_PATHS_PLUGINS"

COPY --from=0 /go/src/github.com/grafana/grafana/bin/linux-amd64/grafana-server /go/src/github.com/grafana/grafana/bin/linux-amd64/grafana-cli ./bin/
COPY --from=0 /opt/oracle /opt/oracle
COPY --from=1 /usr/src/app/public ./public
COPY --from=1 /usr/src/app/tools ./tools
COPY tools/phantomjs/render.js ./tools/phantomjs/render.js

EXPOSE 3000

COPY ./packaging/docker/run.sh /run.sh

USER grafana
ENTRYPOINT [ "/run.sh" ]
