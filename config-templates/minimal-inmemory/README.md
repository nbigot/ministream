# Configuration for "minimal-inmemory"

This template is a "quick and easy deploy" one.

There is little to no customization if you want to use it.


## Target usages

This configuration template can be mainly used for:

- local server on developer's local machine
- demo
- proof of concept
- test environment


**This template is not recomanded to be runned on a production environment.**


## Points of interest

### Inmemory

Because it's using the "in memory" storage provider configuration the data are NOT persisted (there is no persistant storage).


All data will be lost when:

- the server is stopped (either gracefully or using a kill signal)
- the server restarted
- the machine is shut down or powered off


### Security

All security mecanism are disabled in this template.
