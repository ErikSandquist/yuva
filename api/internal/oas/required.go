package oas

import (
	"encoding/json"
	"errors"
)

// The generated models decode a missing required field as its zero value; consent must be declared,
// never assumed, so these bodies refuse to decode without it.

func (b *SubscriptionCreate) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "consent"); err != nil {
		return err
	}
	type plain SubscriptionCreate
	return json.Unmarshal(data, (*plain)(b))
}

func (c *SubscriptionConsent) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "confirmed"); err != nil {
		return errors.New("consent." + err.Error())
	}
	type plain SubscriptionConsent
	return json.Unmarshal(data, (*plain)(c))
}

func requireFields(data []byte, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, n := range names {
		if v, ok := fields[n]; !ok || string(v) == "null" {
			return errors.New(n + " is required")
		}
	}
	return nil
}
