package avm

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type FritzBox struct {
	Url     string
	Timeout time.Duration
	Logger  *slog.Logger
}

func NewFritzBox(logger *slog.Logger) *FritzBox {
	return &FritzBox{
		Url:     "http://fritz.box:49000",
		Timeout: 5 * time.Second,
		Logger:  logger,
	}
}

func (fb *FritzBox) GetWanIpv4() (net.IP, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/igdupnp/control/WANIPConn1", fb.Url), bytes.NewBufferString(soapGetWanIp))

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "text/xml; charset=utf-8;")
	request.Header.Set("SoapAction", "urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress")

	client := &http.Client{
		Timeout: fb.Timeout,
	}

	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	ip, err := parseGetExternalIPAddressResponse(body)

	if err != nil {
		return nil, err
	}

	return ip, nil
}

func (fb *FritzBox) GetwanIpv6() (net.IP, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/igdupnp/control/WANIPConn1", fb.Url), bytes.NewBufferString(soapGetWanIp))

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "text/xml; charset=utf-8;")
	request.Header.Set("SoapAction", "urn:schemas-upnp-org:service:WANIPConnection:1#X_AVM_DE_GetExternalIPv6Address")

	client := &http.Client{
		Timeout: fb.Timeout,
	}

	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	ip, err := parseGetExternalIPv6Address(body)

	if err != nil {
		return nil, err
	}

	return ip, nil
}

func (fb *FritzBox) GetIpv6Prefix() (*net.IPNet, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/igdupnp/control/WANIPConn1", fb.Url), bytes.NewBufferString(soapGetWanIp))

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "text/xml; charset=utf-8;")
	request.Header.Set("SoapAction", "urn:schemas-upnp-org:service:WANIPConnection:1#X_AVM_DE_GetIPv6Prefix")

	client := &http.Client{
		Timeout: fb.Timeout,
	}

	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	ipNet, err := parseGetIPv6Prefix(body)

	if err != nil {
		return nil, err
	}

	return ipNet, nil
}
