# Configuration for "secured-jsonfilestorage"

This template required you to make extra steps for configuration.



## Target usages

This configuration template can be mainly used for:

- production environment



## Points of interest

### Persistant storage

Because it's using the "json file" storage provider configuration the data are persisted on disk as jsonline files.

In the *config.yaml* file you must pay attention and configure the section:

```yaml
storage:
    type: "JSONFile"
    jsonfile:
        dataDirectory: "/app/data/storage"
```

The *dataDirectory* value must be an existing directory with enough rights for the program to read and write into.

Tip: if you are using Docker then you may map a volume to the container at this specific directory path.


### Security

You need to configure:

- the SSL certificate (in the *certs/* sub-directory)
- the passwords (in the *data/secrets/ sub-directory)
  - file *data/secrets/rbac.json* contains the list of roles
  - file *data/secrets/secrets.json* contains the list of login and hashed passwords

Please read the README.md files in those sub-directories.
