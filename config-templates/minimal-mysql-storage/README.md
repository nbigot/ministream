# Configuration for "minimal-mysql-storage"

This template is a "quick and easy deploy" one.

There is little customization if you want to use it.


## Target usages

This configuration template can be mainly used for:

- local server on developer's local machine
- demo
- proof of concept
- test environment


**This template is not recomanded to be runned on a production environment.**


## Points of interest

### Persistant storage

Because it's using the "MySQL" storage provider configuration the data are persisted on the MySQL server.

In the *config.yaml* file you must pay attention and configure the section:

```yaml
storage:
    type: "MySQL"
    mysql:
        dsn: "ministream:ministream@tcp(localhost:3306)/ministream?tls=skip-verify"
        connMaxLifetime: 0
        maxOpenConns: 3
        maxIdleConns: 3
        schemaName: "ministream"
        catalogTableName: "streams"			
```

Tip: if you are using Docker then you may map a volume to persis mysql data.


### Security

All security mecanism are disabled in this template.
