# docker run --rm --pid=host --network=host eparis/remote-shell:latest

FROM rhel7:latest

MAINTAINER Eric Paris <eparis@redhat.com>

#EXPOSE 12021

CMD ["/server"]

ADD config/ /etc/remote-shell/
ADD bin/server /server
ADD bin/client /static/client
RUN chmod +x /server /static/client
