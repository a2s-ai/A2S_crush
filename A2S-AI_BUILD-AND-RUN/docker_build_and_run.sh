#!/bin/sh

docker build -t crush .

docker run \
       -it \
       --rm \
       -v /etc/hosts:/etc/hosts:ro \
       crush

# EOF
