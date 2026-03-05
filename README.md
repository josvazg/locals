# Locals

Locals is a lightweight developer tool designed to make local service development seamless. It provides a local DNS server and a transparent web proxy so you can access your containers or local processes via custom .locals domains with automatic SSL.

## Purpose
- *Vanity Domains*: Stop using localhost:8080. Use api.locals or frontend.locals.
- *Automatic SSL*: Seamless integration with mkcert so your browser and CLI tools trust your local HTTPS traffic.
- *Transparent Proxy*: Routes traffic from standard ports (80/443) to your high-port dev services without manual proxy configuration.
- *CLI Friendly*: Injects the necessary CA bundles into your shell session so curl and git work out of the box.

## Prerequisites

The locals tool requires `mkcert` to be installed on your system. You can get it from [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert)

The `:443` *tcp* and `:53` *udp* ports must not be in use. If you have a service in `127.0.0.1:53` it is fine, `locals dns` listends on `127.1.2.3:53` instead, but if a service (such as dnsmasq) is listening on `0.0.0.0:53`, locals dns will not work. 

## How to Use

### Start the Services

```shell
locals on
```

Runs the background services (DNS and Web Proxy) and redirects `/etc/resolv.conf` to use `locals dns` first. This requires sudo to do a bind mount and to bind to ports 53, 80, and 443 and to mount the resolver.

Note the original `/etc/resolv.conf` is never written to, only bind mounted on top. All `locals on` changes are ephemeral and would go away after a restart .

### Add a Route

```shell
locals add registry.locals localhost:5000
```

Creates a JSON web config in `~/.config/locals/web/myservice.json`:

Now, `https://myservice.locals` will point to your service at `localhost:9876`

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

Undoes the bind mount and stops the services. The system is back to normal and the original `/etc/resolv.conf` is back in place.

## Tested Operating Systems

- NixOS
