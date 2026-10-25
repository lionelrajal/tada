package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// encodeJSON writes v as a single compact JSON document followed by a newline,
// so a consumer reads one complete value and nothing else on stdout.
func encodeJSON(out io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	_, err = fmt.Fprintf(out, "%s\n", data)
	return err
}
