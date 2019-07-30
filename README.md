# AVM FRITZ!Box CloudFlare DNS-service

This project has some simple goals:

- Offer a slim service without any additional service requirements
- Allow for two different combined strategies: Polling (through FRITZ!Box SOAP-API) and Pushing (FRITZ!Box Custom-DynDns setting).
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

| Variable name | Description |
| --- | --- |
| DYNDNS_SERVER_BIND | required, network interface to bind to, i.e. `:8080` |
| DYNDNS_SERVER_USERNAME | optional, username for the DynDNS service |
| DYNDNS_SERVER_PASSWORD | optional, password for the DynDNS service |

Now configure the FRITZ!Box router to push IP changes towards this service. Log into the admin panel and go to
`Internet > Shares > DynDNS tab` and setup a  `Custom` provider:

| Property | Description / Value |
| --- | --- |
| Update-URL | http://[server-ip]/ip?v4=\<ipaddr\>&v6=\<ip6addr\> |
| Domain | Leave blank, no meaning |
| Username | Leave blank if `DYNDNS_SERVER_USERNAME` env is unset |
| Password | Leave blank if `DYNDNS_SERVER_PASSWORD` env is unset |

If you specified credentials you need to append them as additional GET parameters into the Update-URL like `&username=user&password=pass"`.

### FRITZ!Box polling

You can use this strategy if you have:

- no access to the admin panel of the FRITZ!Box router.
- for whatever reasons the router can not push towards this service, but we can poll from it.
- you do not trust pushing

In your `.env` file or your system environment variables you can be configured:

| Variable name | Description |
| --- | --- |
| FRITZBOX_ENDPOINT_URL | optional, how can we reach the router, i.e. `http://fritz.box:49000`, the port should be 49000 anyway. |
| FRITZBOX_ENDPOINT_TIMEOUT | optional, a duration we give the router to respond, i.e. `10s`. |
| FRITZBOX_ENDPOINT_INTERVAL | optional, a duration how often we want to poll the WAN IPs from the router, i.e. `120s` |

You can try the endpoint URL in the browser to make sure you have the correct port, you should receive an `404 ERR_NOT_FOUND`. 

## Cloudflare setup

Login to the cloudflare dashboard, go to your `Profile` and switch over to `API Tokens` and use the `Global API Key`.
I tried to work with the BETA API Tokens but never could the permissions to be right for mapping and updating DNS entries in zones.

In your `.env` file or your system environment variables you can be configured:

| Variable name | Description |
| --- | --- |
| CLOUDFLARE_API_EMAIL | required, your CloudFlare account email |
| CLOUDFLARE_API_KEY | required, your CloudFlare Global API key |
| CLOUDFLARE_ZONES_IPV4 | comma-separated list of domains to update with new IPv4 addresses |
| CLOUDFLARE_ZONES_IPV6 | comma-separated list of domains to update with new IPv6 addresses |

This service allows to update multiple records, an advanced example would be:

```env
CLOUDFLARE_ZONES_IPV4=ipv4.example.com,ip.example.com,server-01.dev.local
CLOUDFLARE_ZONES_IPV6=ipv6.example.com,ip.example.com,server-01.dev.local
```

Considering the example call `http://192.168.0.2/ip?v4=127.0.0.1&v6=::1` every IPv4 listed zone would be updated to
`127.0.0.1` and every IPv6 listed one to `::1`.

## Docker setup

## Docker build

More raw approach would be to build and run it yourself:

```
docker build -t fritzbox-cloudflare-dyndns .
docker run --rm -it -p 8888:8080 fritzbox-cloudflare-dyndns
```

If you leave `CLOUDFLARE_*` unconfigured, pushing to CloudFlare will be disabled for testing purposes, so try to
trigger it by calling `http://127.0.0.1:8888/ip?v4=127.0.0.1&v6=::1` and review the logs.
