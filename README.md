# AVM FRITZ!Box Cloudflare DNS-service

This project has some simple goals:

- Offer a slim service without any additional service requirements
- Allow for two different combined strategies: Polling (through FRITZ!Box SOAP-API) and Pushing (FRITZ!Box Custom-DynDns
  setting).
- Allow multiple domains to be updated with new A (IPv4) and AAAA (IPv6) records
- Push those IP changes directly to CloudFlare DNS
- Deploy in docker compose

If this fits for you, skim over the CNAME workaround if this is a better solution for you, otherwise feel free to visit
the appropriate strategy section of this document and find out how to configure it correctly.

## CNAME record workaround

Before you try this service evaluate a cheap workaround, as it does not require dedicated hardware to run 24/7:

Have dynamic IP updates by using a CNAME record to your myfritz address, found in `Admin > Internet > MyFRITZ-Account`.
It should look like `[hash].myfritz.net`.

This basic example of a BIND DNS entry would make `intranet.example.com` auto update the current IP:

```
$TTL 60
$ORIGIN example.com.
intranet IN CNAME [hash].myfritz.net
```

Beware that this will expose your account hash to the outside world and depend on AVMs service availability.

## Strategies

### FRITZ!Box pushing

You can use this strategy if you have:

- access to the admin panel of the FRITZ!Box router.
- this services runs on a public interface towards the router.

In your `.env` file or your system environment variables you can be configured:

| Variable name          | Description                                          |
|------------------------|------------------------------------------------------|
| DYNDNS_SERVER_BIND     | required, network interface to bind to, i.e. `:8080` |
| DYNDNS_SERVER_USERNAME | optional, username for the DynDNS service            |
| DYNDNS_SERVER_PASSWORD | optional, password for the DynDNS service            |

Now configure the FRITZ!Box router to push IP changes towards this service. Log into the admin panel and go to
`Internet > Shares > DynDNS tab` and setup a  `Custom` provider:

| Property   | Description / Value                                                                   |
|------------|---------------------------------------------------------------------------------------|
| Update-URL | http://[server-ip]/ip?v4=\<ipaddr\>&v6=\<ip6addr\>&prefix=\<ip6lanprefix\>            |
| Domain     | Enter at least one domain name so the router can probe if the update was successfully |
| Username   | Enter '_' if  `DYNDNS_SERVER_USERNAME` env is unset                                   |
| Password   | Enter '_' if `DYNDNS_SERVER_PASSWORD` env is unset                                    |

If you specified credentials you need to append them as additional GET parameters into the Update-URL
like `&username=<username>&password=<pass>`.

### FRITZ!Box polling

You can use this strategy if you have:

- no access to the admin panel of the FRITZ!Box router.
- for whatever reasons the router can not push towards this service, but we can poll from it.
- you do not trust pushing

In your `.env` file or your system environment variables you can be configured:

| Variable name              | Description                                                                                            |
|----------------------------|--------------------------------------------------------------------------------------------------------|
| FRITZBOX_ENDPOINT_URL      | optional, how can we reach the router, i.e. `http://fritz.box:49000`, the port should be 49000 anyway. |
| FRITZBOX_ENDPOINT_TIMEOUT  | optional, a duration we give the router to respond, i.e. `10s`.                                        |
| FRITZBOX_ENDPOINT_INTERVAL | optional, a duration how often we want to poll the WAN IPs from the router, i.e. `120s`                |

You can try the endpoint URL in the browser to make sure you have the correct port, you should receive
an `404 ERR_NOT_FOUND`.

_Because `FRITZBOX_ENDPOINT_URL` is set by default on the docker image, you have to explicitly set it to an empty string
to disable polling_

## Cloudflare setup

To get your API Token do the following: Login to the cloudflare dashboard, go
to `My Profile > API Tokens > Create Token > Edit zone DNS`, give to token some good name (e.g. "DDNS"), add all zones
that the DDNS should be used for, click `Continue to summary` and `Create token`. Be sure to copy the token and add it
to the config, you won't be able to see it again.

In your `.env` file or your system environment variables you can be configured:

| Variable name         | Description                                                       |
|-----------------------|-------------------------------------------------------------------|
| CLOUDFLARE_API_TOKEN  | required, your Cloudflare API Token                               |
| CLOUDFLARE_ZONES_IPV4 | comma-separated list of domains to update with new IPv4 addresses |
| CLOUDFLARE_ZONES_IPV6 | comma-separated list of domains to update with new IPv6 addresses |
| CLOUDFLARE_API_EMAIL  | deprecated, your Cloudflare account email                         |
| CLOUDFLARE_API_KEY    | deprecated, your Cloudflare Global API key                        |

This service allows to update multiple records, an advanced example would be:

```env
CLOUDFLARE_ZONES_IPV4=ipv4.example.com,ip.example.com,server-01.dev.local
CLOUDFLARE_ZONES_IPV6=ipv6.example.com,ip.example.com,server-01.dev.local
```

Considering the example call `http://192.168.0.2:8080/ip?v4=127.0.0.1&v6=::1` every IPv4 listed zone would be updated to
`127.0.0.1` and every IPv6 listed one to `::1`.

## Register IPv6 for another device (port-forwarding)

IPv6 port-forwarding works differently and so if you want to use it you have to add the following configuration.

Warning: `FRITZBOX_ENDPOINT_URL` has to be set for this to work.

To access a device via IPv6 you need to add it's global IPv6 address to cloudflare, for this to be calculated you need
to find out the local part of it's IP.
You can find out the local part of a device's IP, by going to the device's settings and looking at
the `IPv6 Interface-ID`.
It should look something like this: `::1234:5678:90ab:cdef`.
Sometimes the FritzBox seems to use a subnet, so you might need to add change it from something
like `::1234:5678:90ab:cdef` to `::1:1234:5678:90ab:cdef`

| Variable name             | Description                                     |
|---------------------------|-------------------------------------------------|
| DEVICE_LOCAL_ADDRESS_IPV6 | required, enter the local part of the device IP |

## Docker compose setup

Here is an example `docker-compose.yml` with all features activated:

```
version: '3.7'

services:
  updater:
    image: ghcr.io/cromefire/fritzbox-cloudflare-dyndns:1
    network_mode: host
    # build:
    #   context: .
    environment:
      - FRITZBOX_ENDPOINT_URL=http://fritz.box:49000
      - FRITZBOX_ENDPOINT_TIMEOUT=30s
      - FRITZBOX_ENDPOINT_INTERVAL=3s
      - CLOUDFLARE_API_EMAIL=max@example.com
      - CLOUDFLARE_API_KEY=demo
      - CLOUDFLARE_ZONES_IPV4=test.example.com
      - CLOUDFLARE_ZONES_IPV6=test.example.com
```

Now we could configure the FRITZ!Box
to `http://[docker-host-ip]:49000/ip?v4=<ipaddr>&v6=<ip6addr>&prefix=<ip6lanprefix>` and it should trigger the update
process.

## Docker build

A pre-built docker image is also available on this
GitHub [repository](https://github.com/cromefire/fritzbox-cloudflare-dyndns/pkgs/container/fritzbox-cloudflare-dyndns)
as `ghcr.io/cromefire/fritzbox-cloudflare-dyndns:<version>`.
The version is something like `1.2` (you can leave out the patch version), please don't use `latest` directly, as it may
break at any point with a major release.

You can use it with compose like this:

```yaml
name: "dyndns"
services:
  updater:
    image: "ghcr.io/cromefire/fritzbox-cloudflare-dyndns:<version>"
    env_file: ./updater.env
    restart: unless-stopped
    ports:
      - 8080/tcp
```

With your secret configure in the `updater.env` file next to it (as `SOME_VARIABLE=<valua>`).

The more raw approach would be to build and run it yourself:

```
docker build -t fritzbox-cloudflare-dyndns .
docker run --rm -it -p 8888:8080 fritzbox-cloudflare-dyndns
```

If you leave `CLOUDFLARE_*` unconfigured, pushing to CloudFlare will be disabled for testing purposes, so try to
trigger it by calling `http://127.0.0.1:8888/ip?v4=127.0.0.1&v6=::1` and review the logs.
