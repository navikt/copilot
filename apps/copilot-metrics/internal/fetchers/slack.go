package fetchers

import (
	"errors"
	"net/url"
)

func SafeTransportError(err error) error {
	var transport *url.Error
	if errors.As(err, &transport) {
		return transport.Err
	}
	return err
}
