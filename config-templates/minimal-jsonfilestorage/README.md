# Configuration for "minimal-jsonfilestorage"

This template is a "quick and easy deploy" one.

There is little customization if you want to use it.


## Target usages

This configuration template can be mainly used for:

- local server on developer's local machine
- demo
- proof of concept
- test environment
- staging environment


**This template is not recomanded to be runned on a production environment.**


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

All security mecanism are disabled in this template.
