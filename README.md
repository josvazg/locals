# Locals

Locals is a lightweight developer tool designed to make local service development seamless. It provides a local DNS server and a transparent web proxy so you can access your containers or local processes via custom `.locals` domains with automatic SSL.

## Purpose
- *Vanity Domains*: Stop using `localhost:8080`. Use `api.locals` or `frontend.locals`.
- *Automatic SSL*: Seamless integration with mkcert so your browser and CLI tools trust your local HTTPS traffic.
- *Transparent Proxy*: Routes traffic from standard ports (HTTPS on 443) to your high-port dev services without manual per-app proxy configuration.
- *CLI Friendly*: Injects the necessary CA bundles into your shell session so curl and git work out of the box.

## Permissions and trust

`locals` is aimed at **local development**. You should understand what it does before running it on a machine you care about.

- **`locals on`** runs privileged steps via **`sudo`**: it starts the embedded DNS and HTTPS proxy as root (e.g. `nohup` + reserved ports), and it **changes how your OS resolves DNS** so `*.locals` queries hit Locals first.
- **Linux**: may add a **`systemd-resolved`** drop-in (`/etc/systemd/resolved.conf.d/locals.conf`) and restart the service, or **bind-mount** a generated file over **`/etc/resolv.conf`** (the on-disk file is not edited; the mount is removed by `locals off` or a reboot, depending on setup). Locals also reads **`/etc/resolv.conf`** to pick upstream nameservers.
- **macOS**: creates **`/etc/resolver/locals`** and adds a **`lo0`** interface alias so traffic can reach the DNS listener at **`127.1.2.3`**.
- **HTTPS proxy** listens on **`127.0.0.1:443` only** (loopback), not on all interfaces. `*.locals` DNS answers point at **`127.0.0.1`**, so browsers hit that address.

Do not run this tool if you are unwilling to grant **`sudo`** for those operations. Review what `locals on` and `locals off` would run using **`--dryrun`** on the relevant subcommands if you are security-sensitive.

## Prerequisites

The locals tool requires **`mkcert`** on your `PATH`. Install it from [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert).

**Ports:** something else must not already own the addresses Locals needs. In practice:

- **`127.0.0.1:443` (TCP)** must be free for the HTTPS proxy.
- **UDP 53 on `127.1.2.3`** is where **`locals dns`** listens. A resolver bound only to **`127.0.0.1:53`** does not necessarily block that. By contrast, a daemon bound to **`0.0.0.0:53`** (all interfaces) often **does** prevent Locals from binding its DNS port, so stop or reconfigure that service first.

## How it works

All `locals on` changes are ephemeral or easily reversible: Locals does not overwrite your original resolver content by editing it in place where a bind mount is used.

The `locals on`, **`add`**, **`rm`**, and `off` flows run shell commands (see **`--dryrun`** where available) that read and write state under the configuration directory, **`~/.config/locals`** by default (overridable with **`LOCALS_CONFIG_DIR`**).

### Start the Services

```shell
locals on
```

Runs the background services (DNS and HTTPS proxy) and directs the OS to use Locals DNS first. This requires **`sudo`** where noted above.

On **Linux**, the DNS override is implemented either via **systemd-resolved** routing for the `~locals` domain, or by **bind-mounting** a generated file onto **`/etc/resolv.conf`** so it nameservers **`127.1.2.3`** first. The real file under the mount is never modified by Locals directly.

On **macOS**, Locals installs a resolver file so **`*.locals`** uses the Locals DNS server at **`127.1.2.3:53`**, and adds a **`lo0`** alias for **`127.1.2.3`**.

### Add a Route

```shell
locals add myservice.locals localhost:5000
```

Creates a JSON web config under `~/.config/locals/web/` and certificates via mkcert. The first argument is the **host name** for the cert and proxy routing (same value you use in the URL, e.g. **`myservice.locals`**). Then **`https://myservice.locals`** is served by the local proxy and forwarded to **`localhost:5000`**.

### Remove a Route

```shell
locals rm myservice.locals
```

Removes that service’s JSON web config and its cert files under `~/.config/locals/certs/`. After this, **`https://myservice.locals`** is no longer routed by Locals.

### Enable CLI Support

```shell
source <(locals env)
```

As hinted after `locals on`, this wires your **current shell** to trust the local mkcert CA where the OS does not do so system-wide (for example some **NixOS** setups).

### Check Status

```shell
locals status
```

Shows what Locals thinks is active and which domains are mapped.

### Stop and Cleanup

```shell
locals off
```

Stops the services and removes the Linux bind mount or resolver drop-in / macOS resolver and **`lo0`** alias, as applicable.

## Developing

The module targets a **recent Go toolchain** (see **`go.mod`**). If your installed Go is older, the Go command may download the toolchain version listed there automatically.

## Tested Operating Systems

- NixOS
- macOS
- Debian
- Ubuntu
- Fedora
- Arch

GitHub CI only tests **Ubuntu**. **macOS** is not exercised in CI but is intended to work when run locally. For other distros, an [Incus](https://linuxcontainers.org/incus/introduction/) setup on Linux can run **`mage -v testLinuxDistros`** (see `magefiles/mage.go`).
