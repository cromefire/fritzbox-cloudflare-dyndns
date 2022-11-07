package avm

import (
	"bytes"
	"errors"
	"gopkg.in/xmlpath.v2"
	"net"
)

func parseGetExternalIPAddressResponse(xml []byte) (net.IP, error) {
	path := xmlpath.MustCompile("//NewExternalIPAddress")

	root, err := xmlpath.Parse(bytes.NewBuffer(xml))

	if err != nil {
		return nil, err
	}

	v, ok := path.String(root)

	if !ok {
		return nil, err
	}

	ip := net.ParseIP(v)

	if ip == nil {
		return nil, errors.New("failed to parse soap response into IPv4")
	}

	return ip, nil
}

func parseGetExternalIPv6Address(xml []byte) (net.IP, error) {
	pathLifetime := xmlpath.MustCompile("//NewValidLifetime")
	pathAddress := xmlpath.MustCompile("//NewExternalIPv6Address")

	root, err := xmlpath.Parse(bytes.NewBuffer(xml))

	if err != nil {
		return nil, err
	}

	// First check the lifetime as 0 indicates a disabled IPv6 stack
	v, ok := pathLifetime.String(root)

	if !ok {
		return nil, errors.New("xpath not found")
	}

	if v == "0" {
		return nil, nil
	}

	// Now lets parse the actual address
	v, ok = pathAddress.String(root)

	if !ok {
		return nil, errors.New("xpath not found")
	}

	ip := net.ParseIP(v)

	if ip == nil {
		return nil, errors.New("failed to parse soap response into IPv6")
	}

	return ip, nil
}

func parseGetIPv6Prefix(xml []byte) (*net.IPNet, error) {
	pathLifetime := xmlpath.MustCompile("//NewValidLifetime")
	pathPrefix := xmlpath.MustCompile("//NewIPv6Prefix")
	pathPrefixLength := xmlpath.MustCompile("//NewPrefixLength")

	root, err := xmlpath.Parse(bytes.NewBuffer(xml))

	if err != nil {
		return nil, err
	}

	// First check the lifetime as 0 indicates a disabled IPv6 stack
	v, ok := pathLifetime.String(root)

	if !ok {
		return nil, errors.New("xpath not found")
	}

	if v == "0" {
		return nil, nil
	}

	// Now lets parse the actual address
	v, ok = pathPrefix.String(root)

	if !ok {
		return nil, errors.New("xpath not found")
	}

	// Now lets parse the length
	l, ok := pathPrefixLength.String(root)

	if !ok {
		return nil, errors.New("xpath not found")
	}

	_, ipNet, err := net.ParseCIDR(v + "/" + l)

	if err != nil {
		return nil, err
	}

	return ipNet, nil
}
