FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

LABEL name="nuodb/nuodb-operator" \
      vendor="NuoDB, Inc." \
      version="v2.0.2" \
      release="2" \
      summary="NuoDB" \
      description="NuoDB is an distributed, relational database that is easy to set up and scale."

ENV OPERATOR=/usr/local/bin/nuodb-operator \
    USER_UID=1001 \
    USER_NAME=nuodb-operator

COPY licenses /licenses

# install operator binary
COPY build/_output/bin/nuodb-operator ${OPERATOR}

COPY build/etc /usr/local/etc/nuodb-operator

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
