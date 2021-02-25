package cloudwatchhook

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/sirupsen/logrus"
)

// CloudWatchLogsHook is used to store configuration settings for and log messages to Amazon CloudWatch.
type CloudWatchLogsHook struct {
	// required fields
	client            *cloudwatchlogs.Client
	group             string
	stream            string
	nextSequenceToken *string

	// options
	retentionDays int
	kmsKeyID      string
	tags          map[string]string
	logFrequency  time.Duration

	// batching fields
	mutex sync.Mutex
	ch    chan types.InputLogEvent
	err   *error
}

// CloudWatchLogsHookOption is used for creation of optional settings functions.
type CloudWatchLogsHookOption func(*CloudWatchLogsHook)

// NewCloudWatchLogsHook creates a new hook for sending log message to Amazon CloudWatch Logs.
func NewCloudWatchLogsHook(region, group, stream string, options ...CloudWatchLogsHookOption) (
	*CloudWatchLogsHook, error) {

	// create the hook
	var (
		awsConfig aws.Config
		err       error
	)
	if region != "" {
		awsConfig, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	} else {
		awsConfig, err = config.LoadDefaultConfig(context.TODO())
	}
	if err != nil {
		return nil, err
	}
	hook := &CloudWatchLogsHook{
		client:            cloudwatchlogs.NewFromConfig(awsConfig),
		group:             group,
		stream:            stream,
		nextSequenceToken: nil,
		retentionDays:     0,
		kmsKeyID:          "",
		tags:              map[string]string{},
		logFrequency:      0,
		ch:                nil,
		err:               nil,
	}

	// process options
	for _, opt := range options {
		opt(hook)
	}

	// batch the messages
	if hook.logFrequency > 0 {
		hook.ch = make(chan types.InputLogEvent, 10000)
		go hook.putBatch(time.Tick(hook.logFrequency))
	}

	// make sure the group and stream exist; if not, create them
	err = hook.createLogGroup()
	if err != nil {
		return nil, err
	}
	err = hook.createLogStream()
	if err != nil {
		return nil, err
	}
	return hook, nil
}

// WithGroupRetentionDays sets the number of days to retain logs for the log group. This is only valid if the log
// group is being created and does not already exist.
func WithGroupRetentionDays(days int) CloudWatchLogsHookOption {
	return func(h *CloudWatchLogsHook) {
		h.retentionDays = days
	}
}

// WithGroupKmsKeyID sets the Amazon KMS key ID to use for encryption of log data. This is only valid if the log
// group is being created and does not already exist.
func WithGroupKmsKeyID(id string) CloudWatchLogsHookOption {
	return func(h *CloudWatchLogsHook) {
		h.kmsKeyID = id
	}
}

// WithGroupTags sets any tags to associate with the log group. This is only valid if the log group is being created
// and does not already exist.
func WithGroupTags(tags map[string]string) CloudWatchLogsHookOption {
	return func(h *CloudWatchLogsHook) {
		h.tags = tags
	}
}

// WithBatchDuration specifies the frequency with which to upload messages to Amazon CloudWatch. If this option is not
// specified, messages are uploaded immediately.
func WithBatchDuration(frequency time.Duration) CloudWatchLogsHookOption {
	return func(h *CloudWatchLogsHook) {
		h.logFrequency = frequency
	}
}

// Fire is called every time an entry needs to be written to the log.
func (h *CloudWatchLogsHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return fmt.Errorf("Unable to parse entry: %v", err)
	}

	switch entry.Level {
	case logrus.PanicLevel:
		fallthrough
	case logrus.ErrorLevel:
		fallthrough
	case logrus.WarnLevel:
		fallthrough
	case logrus.InfoLevel:
		fallthrough
	case logrus.DebugLevel:
		_, err := h.Write([]byte(line))
		return err
	default:
		return nil
	}
}

// Levels returns the valid levels for the hook.
func (h *CloudWatchLogsHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

// Write handles writing the message to Amazon CloudWatch or to the channel if batching is enabled.
func (h *CloudWatchLogsHook) Write(msg []byte) (int, error) {
	event := types.InputLogEvent{
		Message:   aws.String(string(msg)),
		Timestamp: aws.Int64(int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)),
	}

	// write the message to the batched channel
	if h.ch != nil {
		h.ch <- event
		if h.err != nil {
			lastErr := h.err
			h.err = nil
			return 0, fmt.Errorf("%v", *lastErr)
		}
		return len(msg), nil
	}

	// write the message directly to Amazon CloudWatch
	h.mutex.Lock()
	defer h.mutex.Unlock()
	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     []types.InputLogEvent{event},
		LogGroupName:  aws.String(h.group),
		LogStreamName: aws.String(h.stream),
		SequenceToken: h.nextSequenceToken,
	}
	result, err := h.client.PutLogEvents(context.TODO(), input)
	if err != nil {
		return 0, err
	}
	h.nextSequenceToken = result.NextSequenceToken
	return len(msg), nil
}

// createLogGroup will create the CloudWatch log group if it does not exist already
func (h *CloudWatchLogsHook) createLogGroup() error {
	// find any existing group and return it
	group, err := h.findLogGroup()
	if err != nil {
		return err
	}
	if group != nil {
		return nil
	}

	// create the group
	input := &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(h.group),
	}
	if len(h.tags) > 0 {
		input.Tags = h.tags
	}
	if h.kmsKeyID != "" {
		input.KmsKeyId = aws.String(h.kmsKeyID)
	}
	_, err = h.client.CreateLogGroup(context.TODO(), input)
	if err != nil {
		return err
	}
	return nil
}

// createLogStream will create the CloudWatch log group stream if it does not exist already.
func (h *CloudWatchLogsHook) createLogStream() error {
	// find any existing stream and return it
	stream, err := h.findLogStream()
	if err != nil {
		return err
	}
	if stream != nil {
		return nil
	}

	// create the stream
	input := &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(h.group),
		LogStreamName: aws.String(h.stream),
	}
	_, err = h.client.CreateLogStream(context.TODO(), input)
	if err != nil {
		return err
	}

	// find the stream so we update the current upload sequence token
	_, err = h.findLogStream()
	if err != nil {
		return err
	}
	return nil
}

// findLogGroup finds the hook log group, if it exists. If it does not, it will return nil with no errors.
func (h *CloudWatchLogsHook) findLogGroup() (*types.LogGroup, error) {
	var nextToken *string = nil
	for {
		result, err := h.client.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(h.group),
			NextToken:          nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, group := range result.LogGroups {
			if aws.ToString(group.LogGroupName) == h.group {
				return &group, nil
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}
	return nil, nil
}

// findLogStream finds the hook log stream, if it exists. If it does not, it will return nil with no errors.
func (h *CloudWatchLogsHook) findLogStream() (*types.LogStream, error) {
	var nextToken *string = nil
	for {
		result, err := h.client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName:        aws.String(h.group),
			LogStreamNamePrefix: aws.String(h.stream),
			NextToken:           nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, stream := range result.LogStreams {
			if aws.ToString(stream.LogStreamName) == h.stream {
				h.nextSequenceToken = stream.UploadSequenceToken
				return &stream, nil
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}
	return nil, nil
}

// putBatch is responsible for batching log events and sending them on a set frequency.
func (h *CloudWatchLogsHook) putBatch(ticker <-chan time.Time) {
	var batch []types.InputLogEvent
	size := 0
	for {
		select {
		case p := <-h.ch:
			messageSize := len(*p.Message) + 26
			if size+messageSize > 1048576 || len(batch) == 10000 {
				go h.sendBatch(batch)
				batch = nil
				size = 0
			}
			batch = append(batch, p)
			size += messageSize

		case <-ticker:
			go h.sendBatch(batch)
			batch = nil
			size = 0
		}
	}
}

// sendBatch sends the batch of log events to Amazon CloudWatch.
func (h *CloudWatchLogsHook) sendBatch(batch []types.InputLogEvent) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// nothing to send
	if len(batch) == 0 {
		return
	}

	// send events
	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     batch,
		LogGroupName:  aws.String(h.group),
		LogStreamName: aws.String(h.stream),
		SequenceToken: h.nextSequenceToken,
	}
	result, err := h.client.PutLogEvents(context.TODO(), input)
	if err != nil {
		h.err = &err
	} else {
		h.nextSequenceToken = result.NextSequenceToken
	}
}
