
package validator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

var ErrEmptyPhone = errors.New("phone number is empty")


func ToE164(raw string, defaultRegion string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrEmptyPhone
	}

	num, err := phonenumbers.Parse(raw, defaultRegion)
	if err != nil {
		return "", fmt.Errorf("parse phone number %q: %w", raw, err)
	}
	
	if !phonenumbers.IsValidNumber(num) {
		return "", fmt.Errorf("phone number %q is not valid for region %q", raw, defaultRegion)
	}

	return phonenumbers.Format(num, phonenumbers.E164), nil
}


func FromWaID(waID string) (string, error) {
	waID = strings.TrimSpace(waID)
	if waID == "" {
		return "", ErrEmptyPhone
	}
	if !strings.HasPrefix(waID, "+") {
		waID = "+" + waID
	}
	return ToE164(waID, "")
}


func RegionFor(e164 string) string {
	num, err := phonenumbers.Parse(e164, "")
	if err != nil {
		return ""
	}
	return phonenumbers.GetRegionCodeForNumber(num)
}
