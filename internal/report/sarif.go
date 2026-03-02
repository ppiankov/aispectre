package report

import (
	"errors"
	"io"
)

// ErrUnsupported indicates the SARIF format is not yet implemented.
var ErrUnsupported = errors.New("sarif: output format not yet implemented")

type sarifReporter struct{}

func (r *sarifReporter) Render(_ io.Writer, _ Report) error {
	return ErrUnsupported
}
