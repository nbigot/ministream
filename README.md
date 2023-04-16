# Ministream

[![Go Report Card](https://goreportcard.com/badge/github.com/nbigot/ministream)](https://goreportcard.com/report/github.com/nbigot/ministream)
[![license](https://img.shields.io/github/license/nbigot/ministream)](https://github.com/nbigot/ministream/blob/main/LICENSE)


## Overview

Ministream is an open-source real-time data stream service.
Ministream is well suited for event driven architectures.

Ministream is simple and straightforward, it runs on a single server and has no dependencies over third parties.
Events records are pushed and pulled by a simple HTTP request.
It it served by it's own HTTP(s) server and stores data on json files.
Ministream can easily fit in a standalone docker container.
Ministream also provides a complete web api to manage the server.


## Quick install

### Install the docker image

Use the official docker image: `ghcr.io/nbigot/ministream:latest`

[Packages and versions](https://github.com/nbigot/ministream/pkgs/container/ministream)

Install from the command line:

```sh
$ docker pull ghcr.io/nbigot/ministream:latest
$ docker run --name ministream -it -p 8080:8080 ghcr.io/nbigot/ministream:latest
```


#### Docker quick tips

The docker image `ghcr.io/nbigot/ministream:latest` is a multi-arch image that contains images for:
- linux amd64 v2
- linux amd64 v3
- linux arm64
- linux arm v7


```sh
$ docker manifest inspect ghcr.io/nbigot/ministream:latest
```


### Install by building your own docker image

#### Download source code

```sh
$ git clone https://github.com/nbigot/ministream.git
```


#### Build a docker image

```sh
$ cd ministream
$ docker build -t nbigot/ministream .
```


#### Start a docker container

```sh
$ docker run --name ministream -it -p 8080:8080 nbigot/ministream
```


### Install by compiling source code

#### Download source code

```sh
$ git clone https://github.com/nbigot/ministream.git
```


#### Compile

```sh
$ cd ministream
$ go build cmd/ministream/ministream.go
```


#### Configure

Edit the file *config-templates/docker/config/config.yaml*

Pay attention to the directory paths in the config file.


#### Run ministream

```sh
$ ministream -config config-templates/docker/config/config.yaml
```


## Ministream quick tips

Let's assume Ministream is running and listening on the tcp port 8080.

Run simple commands:

```sh
$ curl http://localhost:8080
Welcome to ministream!

$ curl http://localhost:8080/api/v1/utils/ping
ok
```


## Contribution guidelines

If you want to contribute to Ministream, be sure to review the [code of conduct](CODE_OF_CONDUCT.md).


## License

This software is licensed under the [MIT](./LICENSE).
