package cloudflare

import cf "github.com/cloudflare/cloudflare-go"

type Updater struct {
	IPv4Zone string
	IPv6Zone string

	init bool
	api *cf.API
}

func NewUpdater() *Updater {
	return &Updater{
		init: false,
	}
}

func (u *Updater) Init(token string) error {
	api, err := cf.NewWithAPIToken(token)

	if err != nil {
		return err
	}

	u.api = api

	u.init = true

	return nil
}
