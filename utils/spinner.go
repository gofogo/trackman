package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Spinner is the main component that runs a process
type Spinner struct {
	UUID string
	Name string

	cmd     string
	args    []string
	timeout time.Duration
	workdir string
	step    Step
}

// NewSpinnerForStep creates a new instance of Spinner based on the Options
func NewSpinnerForStep(ctx context.Context, step Step) (*Spinner, error) {
	spinner := newSpinnerForStep(ctx, step)
	spinner.expandEnvVars(ctx)
	err := spinner.parseArgs(ctx)
	if err != nil {
		return nil, err
	}

	return spinner, nil
}

// NewSpinnerForProbe creates a new instance of Spinner based on the Options
func NewSpinnerForProbe(ctx context.Context, step Step) (*Spinner, error) {
	spinner := newSpinnerForProbe(ctx, step)
	spinner.expandEnvVars(ctx)
	err := spinner.parseArgs(ctx)
	if err != nil {
		return nil, err
	}

	return spinner, nil
}

func newSpinnerForStep(ctx context.Context, step Step) *Spinner {
	if step.options == nil {
		panic("no options")
	}

	return &Spinner{
		UUID:    uuid.New().String(),
		Name:    step.Name,
		cmd:     step.Command,
		args:    step.Args,
		step:    step,
		workdir: step.Workdir,
	}
}

func newSpinnerForProbe(ctx context.Context, step Step) *Spinner {
	if step.options == nil {
		panic("no options")
	}

	return &Spinner{
		UUID:    uuid.New().String(),
		Name:    fmt.Sprintf("%s.probe", step.Name),
		cmd:     step.Probe.Command,
		args:    step.Probe.Args,
		step:    step,
		workdir: step.Workdir,
	}
}

func (s *Spinner) expandEnvVars(ctx context.Context) {
	expandedCommand := os.ExpandEnv(s.step.Command)
	s.cmd = expandedCommand

	for idx, item := range s.args {
		s.args[idx] = os.ExpandEnv(item)
	}

	if s.step.workflow == nil {
		panic("no workflow")
	}

	if s.step.workflow.options == nil {
		panic("no workflow option")
	}

	if s.step.Timeout != nil {
		s.timeout = *s.step.Timeout
	} else {
		s.timeout = s.step.workflow.options.Timeout
	}

	if s.workdir != "" {
		s.workdir = os.ExpandEnv(s.workdir)
	}
}

func (s *Spinner) parseArgs(ctx context.Context) error {

	for idx, arg := range s.step.Args {
		buf := &bytes.Buffer{}
		tmpl, err := template.New("t1").Parse(arg)
		if err != nil {
			return err
		}

		err = tmpl.Execute(buf, s.step)
		if err != nil {
			return err
		}

		s.args[idx] = buf.String()
	}

	return nil

}

// Run runs the process required
func (s *Spinner) Run(ctx context.Context) error {
	s.push(ctx, NewEvent(s, EventRunRequested, nil))

	cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, ctx = LoggerContext(ctx)

	// add this spinner to the context for the log writers
	ctx = context.WithValue(ctx, CtxSpinner, s)

	outChannel := NewLogWriter(ctx, logrus.DebugLevel)
	errChannel := NewLogWriter(ctx, logrus.ErrorLevel)

	cmd := exec.CommandContext(cmdCtx, s.cmd, s.args...)
	cmd.Stderr = errChannel
	cmd.Stdout = outChannel
	cmd.Dir = s.workdir

	err := cmd.Start()
	if err != nil {
		s.push(ctx, NewEvent(s, EventRunError, nil))

		return err
	}

	s.push(ctx, NewEvent(s, EventRunStarted, nil))

	if err := cmd.Wait(); err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			s.push(ctx, NewEvent(s, EventRunTimeout, nil))

			return fmt.Errorf("Step %s timed out after %s", s.step.Name, s.timeout)
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				s.push(ctx, NewEvent(s, EventRunFail, status))
				return exitErr
			}
		} else {
			// wait error
			s.push(ctx, NewEvent(s, EventRunWaitError, s))

			return exitErr
		}
	}

	s.push(ctx, NewEvent(s, EventRunSuccess, nil))

	return nil
}

func (s *Spinner) push(ctx context.Context, event *Event) {
	err := s.step.options.Notifier(ctx, event)
	if err != nil {
		fmt.Println(err)
	}
}
