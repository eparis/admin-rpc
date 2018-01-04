# docker run --rm --pid=host --network=host eparis/access-daemon:latest

FROM rhel7:latest

MAINTAINER Eric Paris <eparis@redhat.com>

# Runs on port 80, 
#EXPOSE 3306

CMD ["/access-daemon"]

ADD config /etc/access-daemon
ADD ./daemon/daemon /access-daemon
ADD ./client/client /static/ops-client
RUN setcap cap_net_bind_service=ep /access-daemon && chmod +x /access-daemon /static/ops-client
