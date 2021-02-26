package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

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

	args := []cloudwatchhook.CloudWatchLogsHookOption{}
	retentionPeriod := os.Getenv("AWS_CLOUDWATCH_LOG_RETENTION_DAYS")
	if retentionPeriod != "" {
		days, err := strconv.ParseInt(retentionPeriod, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: AWS_CLOUDWATCH_LOG_RETENTION_DAYS must be an integer")
			os.Exit(1)
		}
		args = append(args, cloudwatchhook.WithGroupRetentionDays(int32(days)))
	}

	batchDuration := os.Getenv("AWS_CLOUDWATCH_LOG_BATCH_DURATION")
	if batchDuration != "" {
		duration, err := time.ParseDuration(batchDuration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: AWS_CLOUDWATCH_LOG_BATCH_DURATION must be a valid duration")
			os.Exit(1)
		}
		args = append(args, cloudwatchhook.WithBatchDuration(duration))
	}

	tags := os.Getenv("AWS_CLOUDWATCH_LOG_GROUP_TAGS")
	if tags != "" {
		kvPair := map[string]string{}
		for _, tag := range strings.Split(tags, ",") {
			kv := strings.Split(tag, "=")
			if len(kv) == 2 {
				kvPair[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			} else {
				kvPair[strings.TrimSpace(kv[0])] = ""
			}
		}
		if len(kvPair) > 0 {
			args = append(args, cloudwatchhook.WithGroupTags(kvPair))
		}
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load AWS default configuration: %s", err)
		os.Exit(2)
	}

	hook, err := cloudwatchhook.NewCloudWatchLogsHook(cfg, group, stream, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create hook: %s", err)
		os.Exit(3)
	}

	l := logrus.New()
	l.Hooks.Add(hook)
	l.SetOutput(ioutil.Discard)
	l.SetFormatter(&logrus.JSONFormatter{})

	for i := 0; i < 10; i++ {
		fmt.Println("Sending INFO message")
		l.WithFields(logrus.Fields{
			"event": "testevent",
			"topic": "testtopic",
			"key":   "testkey",
		}).Info("This is a test message")
		time.Sleep(time.Second * 5)
	}
	os.Exit(0)
}
