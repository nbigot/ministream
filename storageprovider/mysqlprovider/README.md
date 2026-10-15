# MySQL storage provider

## Create a simple MySQL configuration for demonstration

Create a MySQL docker container.

```sh
docker run --name mysql-ministream -e MYSQL_ROOT_PASSWORD=my-secret-pw -e MYSQL_DATABASE=ministream -p 3306:3306 -d mysql:8
```

## Run the integration tests

The integration tests run only when the environment variable `MINISTREAM_TEST_MYSQL_DSN` is set
(they drop and create the schema `ministream_gotest`).

```sh
MINISTREAM_TEST_MYSQL_DSN="root:my-secret-pw@tcp(127.0.0.1:3306)/ministream" go test ./storageprovider/mysqlprovider/
```
