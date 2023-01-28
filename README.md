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

### Download source code

```sh
$ git clone https://github.com/nbigot/ministream.git
```


### Compile

```sh
$ cd ministream
$ go build cmd/ministream/ministream.go
```


### Configure

Edit the file *config-templates/docker-minimal/config/config.yaml*

Pay attention to the directory paths in the config file.


### Run ministream

```sh
$ ministream -config config-templates/docker-minimal/config/config.yaml
```


## Docker quick tips

### Download source code

```sh
$ git clone https://github.com/nbigot/ministream.git
```


### Build a docker image

```sh
$ cd ministream
$ docker build -t nbigot/ministream .
```


### Start a docker container

```sh
$ docker run --name ministream -it -p 8080:8080 nbigot/ministream
```


## Contribution guidelines

If you want to contribute to Ministream, be sure to review the [code of conduct](CODE_OF_CONDUCT.md).


## License

This software is licensed under the [MIT](./LICENSE).
