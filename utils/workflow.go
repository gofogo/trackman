package utils

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

// WorkflowOptions provides options for a workflow
type WorkflowOptions struct {
	Notifier func(ctx context.Context, event *Event) error
}

// Workflow is the internal object to hold a workflow file
type Workflow struct {
	Version  string
	Metadata map[string]string
	Steps    []Step

	options *WorkflowOptions
	logger  *logrus.Logger
}

// LoadWorkflowFromBytes loads a workflow from bytes
func LoadWorkflowFromBytes(buff []byte, options *WorkflowOptions) (*Workflow, error) {
	var workflow *Workflow
	err := yaml.Unmarshal(buff, &workflow)
	if err != nil {
		return nil, err
	}

	if options == nil {
		panic("no options")
	}
	if options.Notifier == nil {
		options.Notifier = func(ctx context.Context, event *Event) error {
			fmt.Println(event.String())

			return nil
		}
	}

	workflow.options = options

	return workflow, nil
}

// LoadWorkflowFromReader loads a workflow from an io reader
func LoadWorkflowFromReader(reader io.Reader, options *WorkflowOptions) (*Workflow, error) {
	buff, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return LoadWorkflowFromBytes(buff, options)
}

// Run runs the entire workflow
func (w *Workflow) Run(ctx context.Context) error {
	if ctx.Value(CtxLogger) == nil {
		w.logger = logrus.New()
	} else {
		w.logger = ctx.Value(CtxLogger).(*logrus.Logger)
	}

	// TODO: override if specified
	options := &SpinnerOptions{
		Notifier: w.options.Notifier,
	}

	for _, step := range w.Steps {
		spinner, err := NewSpinner(ctx, step, options)
		if err != nil {
			return err
		}

		err = spinner.Run(ctx)
		if err != nil {
			if step.StopOnFail {
				return err
			}

			w.logger.WithField("process", spinner.Step.Name).Error(err)

		}
	}

	return nil
}
