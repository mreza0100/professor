package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// unmarshalKeepingNumbers is json.Unmarshal for a user-owned file the installer
// rewrites: numbers decode as json.Number, so an integer beyond float64's 2^53
// (a counter, an id) is written back exactly as the user had it. Input
// json.Unmarshal refuses — empty, truncated, or trailing a second value — is
// refused here too.
func unmarshalKeepingNumbers(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("invalid JSON: data after the top-level value")
		}
		return err
	}
	return nil
}

// jsonNumberIs reports whether a decoded JSON value is the number want,
// whichever form decoded it: json.Number from unmarshalKeepingNumbers, or
// float64 from a plain json.Unmarshal. A literal compared against float64 alone
// would call every kept number different and rewrite a converged file forever.
func jsonNumberIs(value any, want float64) bool {
	switch number := value.(type) {
	case json.Number:
		got, err := number.Float64()
		return err == nil && got == want
	case float64:
		return number == want
	}
	return false
}
