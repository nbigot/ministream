# Configure for security

## Configure RBAC

The *rbac.json* file contains the list of role definition and actions allowed for each role.

You can add as many roles as you want.

A user has one or many roles.

A role has one or many rules.

A rule allow one or many actions, and may have an ABAC (attribute base access control).

An ABAC is a deeper restriction access mecanism based on stream properties filtering.

An action almost correspond to a web api method call.

**You may not change this file if you don't need.**


## Configure secrets

The *rbac.json* file contains the list of role definition and actions allowed for each role.

**You MUST change this file!**

You must at leat change the hashed password contained in this file.

The current passwords are the same as login (ex: if login is "demo" then the password in this file is "demo")


### How to generate new hashed passwords

Use the utility program *generatepasswords.go*

Let's say you want the password to be "mysecretpassword".

Example:

```sh
$ go run ministream/cmd/generatepasswords/generatepasswords.go -digest sha512 -iterations 100 -password mysecretpassword
Generate 1 passwords
Digest: sha512 Iterations: 100 Salt: iYmRc3keAHWbGmCdBzsZ Password: mysecretpassword Hash: $pbkdf2-sha512$i=100$iYmRc3keAHWbGmCdBzsZ$8edbe3e0d68ee5a61f3ecc03f43066f7f75a61ebcb72804585e68550b87fe144d7ba51c6730935ff9faf68180d54ec0d9a29965496b13d3fc153fc91ec720bbd
```


Then copy the last part of the output result:

	$pbkdf2-sha512$i=100$iYmRc3keAHWbGmCdBzsZ$8edbe3e0d68ee5a61f3ecc03f43066f7f75a61ebcb72804585e68550b87fe144d7ba51c6730935ff9faf68180d54ec0d9a29965496b13d3fc153fc91ec720bbd


Finally edit the file *secrets.json*:

```json
{
	"demo": "$pbkdf2-sha512$i=100$iYmRc3keAHWbGmCdBzsZ$8edbe3e0d68ee5a61f3ecc03f43066f7f75a61ebcb72804585e68550b87fe144d7ba51c6730935ff9faf68180d54ec0d9a29965496b13d3fc153fc91ec720bbd"
}
```
