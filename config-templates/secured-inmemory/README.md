# Configuration for "secured-inmemory"

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

Security mecanism are enabled:
- authentication using hashed passwords stored in a file
- JWT (JSON Web Token) authentication
- RBAC (role based access control)
- HTTPS support (but disabled by default, you need to enable it and provide your own TLS certificate)
- CORS policy


## Configure

### ⚠️ IMPORTANT: Security Notice

**The files in this directory contain EXAMPLE CREDENTIALS and CERTIFICATES for demonstration purposes only.**

**DO NOT use these in production!** They are:
- Publicly visible in the repository
- Not secure for production use
- Intended only for testing and learning

See [SECURITY_NOTICE.md](../SECURITY_NOTICE.md) for more details.

### Setup Steps

**Before running in any real environment, you MUST:**

1. **Generate your own secrets and RBAC files** using `generatepasswords.go`:
   ```bash
   go run cmd/generatepasswords/generatepasswords.go -digest sha512 -iterations 100 -password YOUR_SECURE_PASSWORD
   ```

2. **Edit the file** *config-templates/secured-inmemory/config/config.yaml*:
   - Pay attention to the directory paths in the config file. (windows paths are different than linux/mac paths)
   - Edit the secrets file to change the default passwords.
     - `secrets.json` file and file path
     - `rbac.json` file and file path

3. **Generate your own TLS certificates** if enabling HTTPS:
   ```bash
   openssl req -x509 -newkey rsa:4096 -keyout server-key.pem -out server.pem -days 365 -nodes
   ```
   - certFile (file `cert.pem`)
   - keyFile (file `key.pem`)
