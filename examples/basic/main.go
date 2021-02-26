package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	cloudwatchhook "github.com/josh-hogle/logrus-cloudwatch-hook"
	"github.com/sirupsen/logrus"
)

func main() {
	group := os.Getenv("AWS_CLOUDWATCH_LOG_GROUP")
	stream := os.Getenv("AWS_CLOUDWATCH_LOG_STREAM")

	if group == "" || stream == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Please set AWS_CLOUDWATCH_LOG_GROUP and AWS_CLOUDWATCH_LOG_STREAM")
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load AWS default configuration: %s", err)
		os.Exit(2)
	}

	hook, err := cloudwatchhook.NewCloudWatchLogsHook(cfg, group, stream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create hook: %s", err)
		os.Exit(3)
	}

	l := logrus.New()
	l.Hooks.Add(hook)
	l.SetOutput(ioutil.Discard)
	l.SetFormatter(&logrus.JSONFormatter{})

	l.WithFields(logrus.Fields{
		"event": "testevent",
		"topic": "testtopic",
		"key":   "testkey",
	}).Info("This is a test message")
	os.Exit(0)
}
