# MySQL storage provider

## Create a simple MySQL configuration for demonstration

Create a MySQL docker container.

```sh
docker run --name mysql-ministream -e MYSQL_ROOT_PASSWORD=my-secret-pw -e MYSQL_DATABASE=ministream -p 3306:3306 -d mysql:8
```
