// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"testing"

	"forgejo.org/modules/nosql"
	queue_mock "forgejo.org/modules/queue/mock"
	"forgejo.org/modules/setting"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type baseRedisUnitTestSuite struct {
	suite.Suite
}

func TestBaseRedis(t *testing.T) {
	suite.Run(t, &baseRedisUnitTestSuite{})
}

func (suite *baseRedisUnitTestSuite) SetupSuite() {
}

func (suite *baseRedisUnitTestSuite) TestBasic() {
	queueName := "test-queue"
	testCases := []struct {
		Name             string
		ConnectionString string
		QueueName        string
		Unique           bool
	}{
		{
			Name:             "unique",
			ConnectionString: "redis://127.0.0.1/0",
			QueueName:        queueName,
			Unique:           true,
		},
		{
			Name:             "non-unique",
			ConnectionString: "redis://127.0.0.1/0",
			QueueName:        queueName,
			Unique:           false,
		},
		{
			Name:             "unique with prefix",
			ConnectionString: "redis://127.0.0.1/0?prefix=forgejo:queue:",
			QueueName:        "forgejo:queue:" + queueName,
			Unique:           true,
		},
		{
			Name:             "non-unique with prefix",
			ConnectionString: "redis://127.0.0.1/0?prefix=forgejo:queue:",
			QueueName:        "forgejo:queue:" + queueName,
			Unique:           false,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.Name, func() {
			queueSettings := setting.QueueSettings{
				Length:  10,
				ConnStr: testCase.ConnectionString,
			}

			// Configure expectations.
			mockRedisStore := queue_mock.NewInMemoryMockRedis()
			redisClient := nosql.NewMockRedisClient(suite.T())

			redisClient.EXPECT().
				Ping(mock.Anything).
				Return(&redis.StatusCmd{}).
				Times(1)
			redisClient.EXPECT().
				LLen(mock.Anything, testCase.QueueName).
				RunAndReturn(mockRedisStore.LLen).
				Times(1)
			redisClient.EXPECT().
				LPop(mock.Anything, testCase.QueueName).
				RunAndReturn(mockRedisStore.LPop).
				Times(1)
			redisClient.EXPECT().
				RPush(mock.Anything, testCase.QueueName, mock.Anything).
				RunAndReturn(func(ctx context.Context, key string, values ...any) *redis.IntCmd {
					return mockRedisStore.RPush(ctx, key, values[0].([]byte))
				}).
				Times(1)

			if testCase.Unique {
				redisClient.EXPECT().
					SAdd(mock.Anything, testCase.QueueName+"_unique", mock.Anything).
					RunAndReturn(func(ctx context.Context, key string, members ...any) *redis.IntCmd {
						return mockRedisStore.SAdd(ctx, key, members[0].([]byte))
					}).
					Times(1)
				redisClient.EXPECT().
					SRem(mock.Anything, testCase.QueueName+"_unique", mock.Anything).
					RunAndReturn(func(ctx context.Context, key string, members ...any) *redis.IntCmd {
						return mockRedisStore.SRem(ctx, key, members[0].([]byte))
					}).
					Times(1)
				redisClient.EXPECT().
					SIsMember(mock.Anything, testCase.QueueName+"_unique", mock.Anything).
					RunAndReturn(func(ctx context.Context, key string, member any) *redis.BoolCmd {
						return mockRedisStore.SIsMember(ctx, key, member.([]byte))
					}).
					Times(2)
			}

			client, err := newBaseRedisGeneric(
				toBaseConfig(queueName, queueSettings),
				testCase.Unique,
				redisClient,
			)
			suite.Require().NoError(err)

			ctx := context.Background()
			expectedContent := []byte("test")

			suite.Require().NoError(client.PushItem(ctx, expectedContent))

			found, err := client.HasItem(ctx, expectedContent)
			suite.Require().NoError(err)
			if testCase.Unique {
				suite.True(found)
			} else {
				suite.False(found)
			}

			found, err = client.HasItem(ctx, []byte("not found content"))
			suite.Require().NoError(err)
			suite.False(found)

			content, err := client.PopItem(ctx)
			suite.Require().NoError(err)
			suite.Equal(expectedContent, content)
		})
	}
}
