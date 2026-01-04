#!/bin/bash
TAG_VERSION="1.0.0"
MINISTREAM_VERSION="v${TAG_VERSION}"
docker build --build-arg MINISTREAM_VERSION=${MINISTREAM_VERSION} -t nbigot/ministream:${TAG_VERSION} .
