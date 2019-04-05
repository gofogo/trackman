package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cloud66/trackman/notifiers"
	"github.com/cloud66/trackman/utils"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the given workflow",
	Run:   runExec,
}

var (
	workflowFile        string
	notificationManager *utils.NotificationManager
)

func init() {
	runCmd.Flags().StringVarP(&workflowFile, "file", "f", "", "workflow file to run")
	rootCmd.AddCommand(runCmd)
}

func runExec(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	notificationManager = utils.NewNotificationManager(ctx, &notifiers.ConsoleNotifier{})
	defer notificationManager.Close(ctx)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		fmt.Println("\nReceived an interrupt, stopping services...")
		cleanup(ctx)
	}()

	options := &utils.Options{
		Sink: &utils.SpinnerSink{
			StdOut: os.Stdout,
			StdErr: os.Stderr,
		},
		NotificationManager: notificationManager,
	}

	err := notificationManager.Start(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	spinner, err := utils.NewSpinner(ctx, "ls -la", options)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = spinner.Run(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Done")
}

func cleanup(ctx context.Context) {
	notificationManager.Stop(ctx)
}
