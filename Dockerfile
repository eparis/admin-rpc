# docker run --rm --pid=host --network=host eparis/admin-rpc:latest

FROM rhel7:latest

MAINTAINER Eric Paris <eparis@redhat.com>

#EXPOSE 12021

CMD ["/server"]

ADD config/ /etc/admin-rpc/
ADD bin/server /server
ADD bin/client /static/client
