# Logrus Hook for CloudWatch Logs

[![godoc reference](https://pkg.go.dev/github.com/josh-hogle/logrus-cloudwatch-hook/?status.png)](https://pkg.go.dev/github.com/josh-hogle/logrus-cloudwatch-hook)
[![license](https://img.shields.io/badge/license-apache-blue.svg)](https://github.com/josh-hogle/logrus-cloudwatch-hook/blob/trunk/LICENSE)
[![support](https://img.shields.io/badge/support-community-purple.svg)](https://github.com/josh-hogle/logrus-cloudwatch-hook)

## Overview

This is a hook for the [Logrus](https://github.com/sirupsen/logrus) logging library for Go which will send log output to Amazon CloudWatch logs using the [AWS Go v2 SDK](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/). It is loosely based on [kdar's hook code](https://github.com/kdar/logrus-cloudwatchlogs) using the v1 SDK.

## Hook Creation and Usage

The `examples` subdirectory contains both basic and more advanced examples on how to create the hook for Logrus. In general, you'll need to follow these steps:

1. Use the AWS SDK to load a specific AWS profile or the default profile configured for the user or process running the code.
2. Optionally use the `With...` functions to configure settings for the CloudWatch log group if does not exist and must be created. Note that the functions have **no effect** on a log group that already exists.
3. Use the `NewCloudWatchLogsHook` function to specify a log group and stream to use in order to create the hook for Logrus. If the log group or stream does not exist, it will be created automatically.
4. Add the hook to the Logrus log object.

## Log Group Options

If the log group does not exist when `NewCloudWatchLogsHook` is called, the group and stream will be created automatically. The options below apply **only** if the group does not exist. They will **not** be applied to an existing group, even if specified.

- `WithGroupRetentionDays(int32)`: Set the retention time of messages logged to the streams within the group. You must specify 0 (never expire), 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827 or 3653, which are the current valid values according to Amazon.
- `WithGroupKmsKeyID(string)`: Encrypt messages sent to the log group using the given ARN of the CMK.
- `WithGroupTags(map[string]string)`: Add the given tags to the group when it is created. Tags must be separated by a comma (,) and in the form `key=value`.

## Batching Messages

By default, log messages are sent immediately to CloudWatch. Under certain circumstances, you may wish to send them in batches instead, especially for applications that have heavy logging. When calling `NewCloudWatchLogsHook` you can use the `WithBatchDuration(time.Duration)` function to specify an arbitrary amount of time between sending messages to CloudWatch. During that period, messages are queued in memory until they are ready to be sent. Be mindful of the amount of memory required by your application for batching messages this way.

## Links

- [Logrus](https://github.com/sirupsen/logrus) 
- [Getting Started with the AWS SDK for Go V2](https://aws.github.io/aws-sdk-go-v2/docs/getting-started/)
- [AWS Go v2 SDK Reference](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/)
