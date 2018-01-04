#!/bin/bash
	
docker rmi $(docker images | grep '<none>' | awk '{print $3}') > /dev/null 2>&1
docker rm $(docker ps -a -q) > /dev/null 2>&1
exit 0
