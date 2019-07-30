package cloudflare

import cf "github.com/cloudflare/cloudflare-go"

type Updater struct {
	api *cf.API
}

func NewUpdater(email string, key string) *Updater {
	u := &Updater{}

	api, err := cf.New(key, email)

	if err != nil {
		panic(err)
	}

	u.api = api

	return u
}
