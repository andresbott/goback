FROM debian:bookworm-slim

RUN mkdir /var/run/sshd; \
    apt install -y openssh-server; \
    apt clean;

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install openssh-server sudo accountsservice

COPY ./dir /data

ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && mkdir -p /var/run/sshd

EXPOSE 22

ENTRYPOINT ["/entrypoint.sh"]