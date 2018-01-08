# docker run --rm --pid=host --network=host eparis/remote-shell:latest

FROM rhel7:latest

MAINTAINER Eric Paris <eparis@redhat.com>

# Runs on port 80, 
#EXPOSE 3306

CMD ["/server"]

ADD config /etc/remote-shell/
ADD bin/server /server
ADD bin/client /static/client
ADD bin/serverKubeConfig /etc/remote-shell/
RUN setcap cap_net_bind_service=ep /server && chmod +x /server /static/client
