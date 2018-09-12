package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/elgs/jsonql"
	"github.com/mumoshu/division/api"
	"github.com/mumoshu/division/framework"
	"os"
	"time"
)

func (p *dynamoResourceDB) Wait(resource, name string, query string, output string, timeout time.Duration, logs bool) error {
	if name == "" {
		return fmt.Errorf("missing resource name")
	}
	r, err := p.wait(resource, name, query, timeout, logs)
	if err != nil {
		return err
	}
	framework.WriteToStdout(r.Format(output))
	return nil
}

func (p *dynamoResourceDB) wait(resource, name string, query string, timeout time.Duration, logs bool) (*api.Resource, error) {
	resources, err := p.get(resource, name, []string{})
	if err != nil {
		return nil, err
	}

	for _, r := range resources {
		matched, err := match(r, query)
		if err != nil {
			return nil, err
		}
		if matched {
			return &r, nil
		}
	}

	rs, es := p.streamedResources(resource, name, []string{})
	to := make(<-chan time.Time)
	if timeout > 0 {
		to = time.After(timeout)
	}
	logMsgCh := make(<-chan *cloudwatchlogs.FilteredLogEvent)
	logErrCh := make(<-chan error)
	if logs {
		logMsgCh, logErrCh = p.logs.read(resource, name, 0, true)
	}
	for {
		select {
		case <-to:
			return nil, fmt.Errorf("timed out")
		case err := <-es:
			return nil, fmt.Errorf("failed streaming: %v", err)
		case err := <-logErrCh:
			return nil, fmt.Errorf("failed streaming logs: %v", err)
		case msg := <-logMsgCh:
			fmt.Fprintf(os.Stderr, "%s", *msg.Message)
		case res := <-rs:
			matched, e := match(*res, query)
			if err != nil {
				return nil, e
			}
			if matched {
				return res, nil
			}
		default:
		}
	}

	return nil, fmt.Errorf("stream stopped unexpectedly: please rerun the div command")
}

func match(resource api.Resource, query string) (bool, error) {
	r := resource.Format("json")

	stringQuery, err := jsonql.NewStringQuery(r)
	if err != nil {
		return false, err
	}

	ret, err := stringQuery.Query(query)
	if err != nil {
		return false, err
	}

	switch ret.(type) {
	case map[string]interface{}:
		return true, nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected type of jsonql query result")
	}
}
