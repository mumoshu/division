// (c) Yury Kozyrev (urakozz)
// MIT License

package stream

import (
	"errors"
	"time"

	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"os"
	"sync"
)

type StreamSubscriber struct {
	dynamoSvc         *dynamodb.DynamoDB
	streamSvc         *dynamodbstreams.DynamoDBStreams
	table             *string
	ShardIteratorType *string
	Limit             *int64
}

func NewStreamSubscriber(
	dynamoSvc *dynamodb.DynamoDB,
	streamSvc *dynamodbstreams.DynamoDBStreams,
	table string) *StreamSubscriber {
	s := &StreamSubscriber{dynamoSvc: dynamoSvc, streamSvc: streamSvc, table: &table}
	s.applyDefaults()
	return s
}

func (r *StreamSubscriber) applyDefaults() {
	if r.ShardIteratorType == nil {
		r.ShardIteratorType = aws.String(dynamodbstreams.ShardIteratorTypeLatest)
	}
}

func (r *StreamSubscriber) SetLimit(v int64) {
	r.Limit = aws.Int64(v)
}

func (r *StreamSubscriber) SetShardIteratorType(s string) {
	r.ShardIteratorType = aws.String(s)
}

func (r *StreamSubscriber) GetStreamDataAsync() (<-chan *dynamodbstreams.Record, <-chan error) {
	ch := make(chan *dynamodbstreams.Record, 1)
	errCh := make(chan error, 1)

	shardsReloadRequests := make(chan struct{}, 1)
	shardsReloadRequests <- struct{}{}

	readingShards := make(map[string]struct{})
	shardProcessingLimit := 10
	shardIteratorInputs := make(chan *dynamodbstreams.GetShardIteratorInput, shardProcessingLimit)
	lock := sync.Mutex{}

	streamArn, err := r.getLatestStreamArn()
	if err != nil {
		errCh <- err
		return ch, errCh
	}
	shards, err := r.getShards(streamArn)
	if err != nil {
		errCh <- err
		return ch, errCh
	}
	numShards := len(shards)
	if numShards > shardProcessingLimit {
		panic(fmt.Errorf("too many shards: div supports up to %d, but there were %d shards", shardProcessingLimit, numShards))
	}
	fmt.Fprintf(os.Stderr, "reading %d shards\n", numShards)
	for _, shard := range shards {
		if _, ok := readingShards[*shard.ShardId]; !ok {
			readingShards[*shard.ShardId] = struct{}{}
			shardIteratorInputs <- &dynamodbstreams.GetShardIteratorInput{
				StreamArn: streamArn,
				ShardId:   shard.ShardId,
				// Read from LATEST
				ShardIteratorType: r.ShardIteratorType,
			}
		}
	}

	go func() {
		tick := time.NewTicker(time.Minute)
		for {
			select {
			case <-tick.C:
				shardsReloadRequests <- struct{}{}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-shardsReloadRequests:
				streamArn, err := r.getLatestStreamArn()
				if err != nil {
					errCh <- err
					return
				}
				shards, err := r.getShards(streamArn)
				if err != nil {
					errCh <- err
					return
				}
				for _, shard := range shards {
					lock.Lock()
					if _, ok := readingShards[*shard.ShardId]; !ok {
						readingShards[*shard.ShardId] = struct{}{}
						shardIteratorInputs <- &dynamodbstreams.GetShardIteratorInput{
							StreamArn: streamArn,
							ShardId:   shard.ShardId,
							// As this shard is created after we start reading, always try to read from TRIM_HORIZON
							// So that we won't miss records that are created before we start reading
							ShardIteratorType: aws.String(dynamodbstreams.ShardIteratorTypeTrimHorizon),
						}
					}
					lock.Unlock()
				}
			}
		}

	}()

	limit := make(chan struct{}, shardProcessingLimit)

	go func() {
		for shardIteratorInput := range shardIteratorInputs {
			limit <- struct{}{}
			go func(sInput *dynamodbstreams.GetShardIteratorInput) {
				err := r.readRecordsFromShardContinuously(sInput, ch)
				if err != nil {
					errCh <- err
				}
				// TODO: think about cleaning list of shards: delete(readingShards, *sInput.ShardId)
				<-limit
			}(shardIteratorInput)
		}
		println("restarting shards reader in 10 seconds...")
		time.Sleep(time.Second * 10)
	}()
	return ch, errCh
}

func (r *StreamSubscriber) getShards(streamArn *string) (ids []*dynamodbstreams.Shard, err error) {
	des, err := r.streamSvc.DescribeStream(&dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	})
	if err != nil {
		return nil, err
	}
	// No shards
	if 0 == len(des.StreamDescription.Shards) {
		return nil, nil
	}

	return des.StreamDescription.Shards, nil
}

func (r *StreamSubscriber) getLatestStreamArn() (*string, error) {
	tableInfo, err := r.dynamoSvc.DescribeTable(&dynamodb.DescribeTableInput{TableName: r.table})
	if err != nil {
		return nil, err
	}
	if nil == tableInfo.Table.LatestStreamArn {
		return nil, errors.New("empty table stream arn")
	}
	return tableInfo.Table.LatestStreamArn, nil
}

func (r *StreamSubscriber) processShardBackport(shardId, lastStreamArn *string, ch chan<- *dynamodbstreams.Record) error {
	return r.readRecordsFromShardContinuously(&dynamodbstreams.GetShardIteratorInput{
		StreamArn:         lastStreamArn,
		ShardId:           shardId,
		ShardIteratorType: r.ShardIteratorType,
	}, ch)
}

func (r *StreamSubscriber) readRecordsFromShardContinuously(input *dynamodbstreams.GetShardIteratorInput, ch chan<- *dynamodbstreams.Record) error {
	iter, err := r.streamSvc.GetShardIterator(input)
	if err != nil {
		return err
	}
	if iter.ShardIterator == nil {
		return nil
	}

	nextIterator := iter.ShardIterator

	for nextIterator != nil {
		recs, err := r.streamSvc.GetRecords(&dynamodbstreams.GetRecordsInput{
			ShardIterator: nextIterator,
			Limit:         r.Limit,
		})
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "TrimmedDataAccessException" {
			//Trying to request data older than 24h, that's ok
			//http://docs.aws.amazon.com/dynamodbstreams/latest/APIReference/API_GetShardIterator.html -> Errors
			return nil
		}
		if err != nil {
			return err
		}

		for _, record := range recs.Records {
			ch <- record
		}

		nextIterator = recs.NextShardIterator

		sleepDuration := time.Second

		// Nil next itarator, shard is closed
		if nextIterator == nil {
			sleepDuration = time.Millisecond * 10
		} else if len(recs.Records) == 0 {
			// Empty set, but shard is not closed -> sleep a little
			sleepDuration = time.Second * 10
		}

		time.Sleep(sleepDuration)
	}
	return nil
}
