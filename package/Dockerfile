FROM ubuntu:16.04
RUN apt-get update && apt-get install -y ca-certificates 

COPY entrypoint.sh /entrypoint.sh
COPY secrets-bridge-v2 /usr/bin/
COPY secrets-bridge-v2-server /usr/bin/

ADD https://raw.githubusercontent.com/rancher/rancher/v1.6/server/bin/update-rancher-ssl /usr/bin/update-rancher-ssl
RUN chmod +x /usr/bin/update-rancher-ssl /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["secrets-bridge-v2"]
