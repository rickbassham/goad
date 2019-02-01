package sqsadapter

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/rickbassham/goad/api"
	uuid "github.com/satori/go.uuid"
)

// Adapter is used to send messages to the queue
type Adapter struct {
	Client   *sqs.SQS
	QueueURL string
}

// DummyAdapter is used to send messages to the screen for testing
type DummyAdapter struct {
	QueueURL string
}

// NewSQSAdapter returns a new sqs adator object
func New(awsConfig *aws.Config, queueURL string) *Adapter {
	return &Adapter{getClient(awsConfig), queueURL}
}

// NewDummyAdaptor returns a new sqs adator object
func NewDummyAdaptor(queueURL string) *DummyAdapter {
	return &DummyAdapter{queueURL}
}

func getClient(awsConfig *aws.Config) *sqs.SQS {
	client := sqs.New(session.New(), awsConfig)
	return client
}

// Receive a result, or timeout in 1 second
func (adaptor Adapter) Receive() []*api.RunnerResult {
	params := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(adaptor.QueueURL),
		MaxNumberOfMessages: aws.Int64(10),
		VisibilityTimeout:   aws.Int64(1),
		WaitTimeSeconds:     aws.Int64(1),
	}
	resp, err := adaptor.Client.ReceiveMessage(params)

	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	if len(resp.Messages) == 0 {
		return nil
	}

	items := resp.Messages
	results := make([]*api.RunnerResult, 0)
	deleteEntries := make([]*sqs.DeleteMessageBatchRequestEntry, 0)
	for _, item := range items {
		result, jsonerr := resultFromJSON(*item.Body)
		if jsonerr != nil {
			fmt.Println(err.Error())
			return nil
		}
		deleteEntries = append(deleteEntries, &sqs.DeleteMessageBatchRequestEntry{
			Id:            aws.String(*item.MessageId),
			ReceiptHandle: aws.String(*item.ReceiptHandle),
		})
		results = append(results, result)
	}

	deleteParams := &sqs.DeleteMessageBatchInput{
		Entries:  deleteEntries,
		QueueUrl: aws.String(adaptor.QueueURL),
	}
	_, delerr := adaptor.Client.DeleteMessageBatch(deleteParams)

	if delerr != nil {
		fmt.Println(delerr.Error())
		return nil
	}

	return results
}

func resultFromJSON(str string) (*api.RunnerResult, error) {
	var result = &api.RunnerResult{
		Statuses: make(map[string]int),
	}
	jsonerr := json.Unmarshal([]byte(str), result)
	if jsonerr != nil {
		return result, jsonerr
	}
	return result, nil
}

func jsonFromResult(result api.RunnerResult) (string, error) {
	data, jsonerr := json.Marshal(result)
	if jsonerr != nil {
		return "", jsonerr
	}
	return string(data), nil
}

// SendResult adds a result to the queue
func (adaptor Adapter) SendResult(result api.RunnerResult) error {
	str, jsonerr := jsonFromResult(result)
	if jsonerr != nil {
		fmt.Println(jsonerr)
		panic(jsonerr)
	}
	params := &sqs.SendMessageInput{
		MessageBody:            aws.String(str),
		MessageGroupId:         aws.String("goad-lambda"),
		MessageDeduplicationId: aws.String(uuid.NewV4().String()),
		QueueUrl:               aws.String(adaptor.QueueURL),
	}
	_, err := adaptor.Client.SendMessage(params)

	return err
}

// SendResult prints the result
func (adaptor DummyAdapter) SendResult(result api.RunnerResult) {
	str, jsonerr := jsonFromResult(result)
	if jsonerr != nil {
		fmt.Println(jsonerr)
		return
	}
	fmt.Println("\n" + str)
}
