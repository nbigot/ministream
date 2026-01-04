# Certificates

The SSL certificate is used only if you configure Ministream as an **HTTPS server**.


## Discalmer

**The certificate files provided here are ONLY for demonstration purpose, do NOT use them on a production environment (it's not safe) !**


## Configuration

In the *config.yaml* file serach for the section:

```yaml
webserver:
    https:
        enable: true
        address: "0.0.0.0:443"
        certFile: "/app/certs/cert.pem"
        keyFile: "/app/certs/key.pem"
```


## Generate an SSL certificate

This directory must have two files:

- cert.pem (the certificate file)
- key.pem (the private key file)


### Generate an ssl certificate with a trusted third-party

You may use https://letsencrypt.org/ to generate a certificate.


### Generate a self signed ssl certificate

If you don't own a certificate you can generate a self signed ssl certificate (beware, this is NOT a best practice).

How to generate a self signed ssl certificate?

You can generate a self signed ssl certificate using openssl:


```sh
$ openssl req -subj "/CN=localhost" -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365
```

Tip: if you are using Windows then use a bash shell to run the openssl command.

Tip: in this example the certificate is only valid for 365 days (it means that you will have to regenerate a new certificate before its expiry date).

Tip: do NOT use a self signed certificate on your production environment (it's not safe).

**DO NOT share nor store your openssl keys on a public repository !**

Those files are there for demonstration only, don't use them on your production environment.

Instead you MUST generate your own certificate (files cert.pem and key.pem) using by example the openssl command above.


## Postman

Postman is helpfull a program for testing web apis.

When using Postman program you must:

Import the *cert.pem* file into postman (menu File --> Settings --> Certificates)

If error then Disable "Enable SSL certificate verification" in postman for self signed certificates.
Set hostname = localhost
