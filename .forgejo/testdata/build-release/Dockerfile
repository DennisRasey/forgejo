FROM code.forgejo.org/oci/alpine:3.19
ARG RELEASE_VERSION=unkown
RUN mkdir -p /app/gitea
RUN ( echo '#!/bin/sh' ; echo "echo forgejo v$RELEASE_VERSION" ) > /app/gitea/gitea ; chmod +x /app/gitea/gitea
