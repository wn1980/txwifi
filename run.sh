#!/bin/bash

#docker build -t wn1980/iotwifi .

docker rm iotwifi

docker run --name iotwifi -it -d \
    --privileged \
    --network=host \
    --cap-add SYS_ADMIN \
    -v $(pwd)/cfg/wificfg.json:/cfg/wificfg.json \
    wn1980/iotwifi

