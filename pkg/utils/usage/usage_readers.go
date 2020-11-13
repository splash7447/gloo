package usage

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/solo-io/reporting-client/pkg/client"
)

const (
	// this is the url for a grpc service
	// note that grpc name resolution is a little different than for a normal HTTP/1.1 service
	// https://github.com/grpc/grpc/blob/master/doc/naming.md
	ReportingServiceUrl = "reporting.corp.solo.io:443"

	// report once per period
	ReportingPeriod = time.Hour * 24

	args = "args"
)

type DefaultUsageReader struct {
}

var _ client.UsagePayloadReader = &DefaultUsageReader{}

// todo, how that this version of GetPayload no longer requires a context, do we still need it in the interface function at all?
func (d *DefaultUsageReader) GetPayload(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

type CliUsageReader struct {
}

var _ client.UsagePayloadReader = &CliUsageReader{}

// when reporting usage, also include the args that glooctl was invoked with
func (c *CliUsageReader) GetPayload(ctx context.Context) (map[string]string, error) {
	argsMap := map[string]string{}

	if len(os.Args) > 1 {
		// don't report the binary name, which will be the first arg
		// it may contain paths on the user's computer
		argsMap[args] = strings.Join(os.Args[1:], "|")
	}

	return argsMap, nil
}
