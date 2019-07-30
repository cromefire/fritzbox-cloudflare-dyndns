# fritzbox-cloudflare-dyndns

## Possible methods

### CNAME record

The most simple way to have dynamic IP updates is to put a CNAME record to your myfritz address, found in `Admin > Internet > MyFRITZ-Account`.
It should look like `[hash].myfritz.net`.

This basic example of a BIND DNS entry would make `intranet.example.com` auto update the current IP: 

```
$TTL 60
$ORIGIN example.com.
intranet IN CNAME [hash].myfritz.net
```

Beware that this will expose your account hash to the outside world and depend on AVMs service availability.

### Fritz!Box polling

### Fritz!Box pushing

## Cloudflare setup

Login to the cloudflare dashboard, go to your `Profile` and switch over to `API Tokens`.

Create a new token with the permission `Zone > DNS > Edit` and limit it to the affected zone like `Include > Specific zone > (your-zone)`.
