# Locals

Locals is a lightweight developer tool designed to make local service development seamless. It provides a local DNS server and a transparent web proxy so you can access your containers or local processes via custom .locals domains with automatic SSL.

## Purpose
- *Vanity Domains*: Stop using localhost:8080. Use api.locals or frontend.locals.
- *Automatic SSL*: Seamless integration with mkcert so your browser and CLI tools trust your local HTTPS traffic.
- *Transparent Proxy*: Routes traffic from standard ports (80/443) to your high-port dev services without manual proxy configuration.
- *CLI Friendly*: Injects the necessary CA bundles into your shell session so curl and git work out of the box.

## Prerequisites

The locals tool requires `mkcert` to be installed on your system. You can get it from [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert)

The `:443` *tcp* and `:53` *udp* ports must not be in use. A service in `127.0.0.1:53` it is fine, because `locals dns` listens on `127.1.2.3:53` instead. On the other hand, some services like DNSMasq, listening on `0.0.0.0:53`will block locals dns from starting. 

## How it works

All `locals on` changes are ephemeral or easily reversible, original configuration is never overwritten.

The `locals on` `add` `rm` and `off` commands produce and execute a bash script at the configuration directory, `~/.config/locals` by default. These script can also be inspected by using the `--dryrun`flag.

### Start the Services

```shell
locals on
```

Runs the background services (DNS and Web Proxy) and redirects your machine to use `locals dns` first. This requires sudo permissions.

On Linux, the dns override is achieved by mount binding `/ets/resolv.conf` pointing to a configuration modified by locals that prepends the locals dns name server at 127.1.2.3 first.
Note the original `/etc/resolv.conf` is never written to, only bind mounted on top. Such change is ephemeral and is overriden by a restart.

On MacOS, the dns overrides implies creating a `/etc/resolver/locals` file that points all *.locals domain names to the locals dns server at 127.1.2.3 port 53. Additionally, an alias is set for link lo0 to IP 127.1.2.3.

### Add a Route

```shell
locals add registry.locals localhost:5000
```

Creates a JSON web config in `~/.config/locals/web/myservice.json`:

Now, `https://myservice.locals` will point to your service at `localhost:5000`

### Remove a Route

```shell
locals rm myservice.locals
```

Removes the JSON web config at `~/.config/locals/web/myservice.json`:

After this `https://registry.locals` is no longer available.

### Enable CLI Support

```shell
source <(locals env)
```

As hinted in the `locals on` command, this enables the user to make curl and other CLI tools trust the local certificates in your current terminal. This is only required in Operating Systems that `mkcert` does not fully support to change the system wide trust source, as happens in NixOS, for instance.

### Check Status

```shell
locals status
```

To see what is running and which domains are mapped.

### Stop and Cleanup

```shell
locals off
```

Stops the services and unmounts the bind mount on Linux or removes the resolver and link alias in MacOS. The system is back to its previous configuration.

## Tested Operating Systems

- NixOS
- MacOS
- Debian

