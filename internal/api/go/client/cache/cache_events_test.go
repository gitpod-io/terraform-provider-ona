package cache_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/client/cache"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -package cache_test -destination cache_mock_test.go github.com/gitpod-io/gitpod-next/api/go/client/cache Invalidator,EventStream

func TestInvalidateFromEventService(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Error         string
		Invalidations []string
		Removals      []string
	}
	tests := []struct {
		Name        string
		Expectation Expectation
		Events      []*v1.WatchEventsResponse
	}{
		{
			Name: "no events",
		},
		{
			Name: "invalidate",
			Expectation: Expectation{
				Invalidations: []string{"foo", "bar"},
			},
			Events: []*v1.WatchEventsResponse{
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_CREATE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_ENVIRONMENT,
					ResourceId:   "foo",
				},
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_UPDATE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_ENVIRONMENT,
					ResourceId:   "bar",
				},
			},
		},
		{
			Name: "remove",
			Expectation: Expectation{
				Removals: []string{"foo"},
			},
			Events: []*v1.WatchEventsResponse{
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_DELETE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_ENVIRONMENT,
					ResourceId:   "foo",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			var act Expectation

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			invalidator := NewMockInvalidator(ctrl)
			invalidator.EXPECT().InvalidateItem(gomock.Any(), gomock.Any()).Return().Do(func(ctx context.Context, key string) { act.Invalidations = append(act.Invalidations, key) }).AnyTimes()
			invalidator.EXPECT().RemoveItem(gomock.Any(), gomock.Any()).Return().Do(func(ctx context.Context, key string) { act.Removals = append(act.Removals, key) }).AnyTimes()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var eventIdx int
			stream := NewMockEventStream(ctrl)
			stream.EXPECT().Receive().DoAndReturn(func() bool {
				if eventIdx >= len(test.Events) {
					cancel()
					return false
				}
				eventIdx++
				return true
			}).AnyTimes()
			stream.EXPECT().Close().AnyTimes()
			stream.EXPECT().Err().Return(nil).AnyTimes()
			stream.EXPECT().Msg().DoAndReturn(func() (*v1.WatchEventsResponse, error) {
				if eventIdx > len(test.Events) {
					return nil, nil
				}
				return test.Events[eventIdx-1], nil
			}).AnyTimes()

			err := cache.InvalidateFromEventService(ctx, &v1.WatchEventsRequest{
				Scope: &v1.WatchEventsRequest_EnvironmentId{
					EnvironmentId: "foo",
				},
			}, func(ctx context.Context, r *connect.Request[v1.WatchEventsRequest]) (cache.EventStream, error) {
				return stream, nil
			}, invalidator)
			if err != nil && !errors.Is(err, context.Canceled) {
				act.Error = err.Error()
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("InvalidateFromEventService() mismatch (-want  got):\n%s", diff)
			}
		})
	}
}

func TestTypedInvalidatorFiltering(t *testing.T) {
	t.Parallel()

	type invocation struct {
		Op  string // "invalidate" or "remove"
		Key string
	}

	tests := []struct {
		Name       string
		FilterType v1.ResourceType
		Expected   []invocation
		Events     []*v1.WatchEventsResponse
	}{
		{
			Name:       "only matching resource types are forwarded",
			FilterType: v1.ResourceType_RESOURCE_TYPE_SERVICE,
			Expected: []invocation{
				{Op: "invalidate", Key: "svc-1"},
			},
			Events: []*v1.WatchEventsResponse{
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_UPDATE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_ENVIRONMENT,
					ResourceId:   "env-1",
				},
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_UPDATE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_SERVICE,
					ResourceId:   "svc-1",
				},
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_UPDATE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_TASK_EXECUTION,
					ResourceId:   "te-1",
				},
			},
		},
		{
			Name:       "delete operations are filtered by type too",
			FilterType: v1.ResourceType_RESOURCE_TYPE_TASK,
			Expected: []invocation{
				{Op: "remove", Key: "task-1"},
			},
			Events: []*v1.WatchEventsResponse{
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_DELETE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_SERVICE,
					ResourceId:   "svc-1",
				},
				{
					Operation:    v1.ResourceOperation_RESOURCE_OPERATION_DELETE,
					ResourceType: v1.ResourceType_RESOURCE_TYPE_TASK,
					ResourceId:   "task-1",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			var actual []invocation
			mock := NewMockInvalidator(ctrl)
			mock.EXPECT().InvalidateItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
				actual = append(actual, invocation{Op: "invalidate", Key: key})
			}).AnyTimes()
			mock.EXPECT().RemoveItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
				actual = append(actual, invocation{Op: "remove", Key: key})
			}).AnyTimes()

			typed := cache.ForResourceTypes(mock, test.FilterType)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			stream := newMockStream(ctrl, test.Events, cancel)

			err := cache.InvalidateFromEventService(ctx, &v1.WatchEventsRequest{
				Scope: &v1.WatchEventsRequest_EnvironmentId{EnvironmentId: "env-1"},
			}, func(ctx context.Context, r *connect.Request[v1.WatchEventsRequest]) (cache.EventStream, error) {
				return stream, nil
			}, typed)
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Expected, actual); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("multiple typed invalidators each receive only their types", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		var svcInvocations, taskInvocations []invocation

		svcMock := NewMockInvalidator(ctrl)
		svcMock.EXPECT().InvalidateItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
			svcInvocations = append(svcInvocations, invocation{Op: "invalidate", Key: key})
		}).AnyTimes()
		svcMock.EXPECT().RemoveItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
			svcInvocations = append(svcInvocations, invocation{Op: "remove", Key: key})
		}).AnyTimes()

		taskMock := NewMockInvalidator(ctrl)
		taskMock.EXPECT().InvalidateItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
			taskInvocations = append(taskInvocations, invocation{Op: "invalidate", Key: key})
		}).AnyTimes()
		taskMock.EXPECT().RemoveItem(gomock.Any(), gomock.Any()).Do(func(_ context.Context, key string) {
			taskInvocations = append(taskInvocations, invocation{Op: "remove", Key: key})
		}).AnyTimes()

		svcTyped := cache.ForResourceTypes(svcMock, v1.ResourceType_RESOURCE_TYPE_SERVICE)
		taskTyped := cache.ForResourceTypes(taskMock, v1.ResourceType_RESOURCE_TYPE_TASK, v1.ResourceType_RESOURCE_TYPE_TASK_EXECUTION)

		events := []*v1.WatchEventsResponse{
			{Operation: v1.ResourceOperation_RESOURCE_OPERATION_UPDATE, ResourceType: v1.ResourceType_RESOURCE_TYPE_SERVICE, ResourceId: "svc-1"},
			{Operation: v1.ResourceOperation_RESOURCE_OPERATION_UPDATE, ResourceType: v1.ResourceType_RESOURCE_TYPE_TASK, ResourceId: "task-1"},
			{Operation: v1.ResourceOperation_RESOURCE_OPERATION_UPDATE, ResourceType: v1.ResourceType_RESOURCE_TYPE_TASK_EXECUTION, ResourceId: "te-1"},
			{Operation: v1.ResourceOperation_RESOURCE_OPERATION_UPDATE, ResourceType: v1.ResourceType_RESOURCE_TYPE_ENVIRONMENT, ResourceId: "env-1"},
			{Operation: v1.ResourceOperation_RESOURCE_OPERATION_DELETE, ResourceType: v1.ResourceType_RESOURCE_TYPE_SERVICE, ResourceId: "svc-2"},
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream := newMockStream(ctrl, events, cancel)

		err := cache.InvalidateFromEventService(ctx, &v1.WatchEventsRequest{
			Scope: &v1.WatchEventsRequest_EnvironmentId{EnvironmentId: "env-1"},
		}, func(ctx context.Context, r *connect.Request[v1.WatchEventsRequest]) (cache.EventStream, error) {
			return stream, nil
		}, svcTyped, taskTyped)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedSvc := []invocation{
			{Op: "invalidate", Key: "svc-1"},
			{Op: "remove", Key: "svc-2"},
		}
		expectedTask := []invocation{
			{Op: "invalidate", Key: "task-1"},
			{Op: "invalidate", Key: "te-1"},
		}

		if diff := cmp.Diff(expectedSvc, svcInvocations); diff != "" {
			t.Errorf("service invalidator mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(expectedTask, taskInvocations); diff != "" {
			t.Errorf("task invalidator mismatch (-want +got):\n%s", diff)
		}
	})
}

// newMockStream creates a mock event stream that yields the given events then cancels the context.
func newMockStream(ctrl *gomock.Controller, events []*v1.WatchEventsResponse, cancel context.CancelFunc) *MockEventStream {
	var eventIdx int
	stream := NewMockEventStream(ctrl)
	stream.EXPECT().Receive().DoAndReturn(func() bool {
		if eventIdx >= len(events) {
			cancel()
			return false
		}
		eventIdx++
		return true
	}).AnyTimes()
	stream.EXPECT().Close().AnyTimes()
	stream.EXPECT().Err().Return(nil).AnyTimes()
	stream.EXPECT().Msg().DoAndReturn(func() (*v1.WatchEventsResponse, error) {
		if eventIdx > len(events) {
			return nil, nil
		}
		return events[eventIdx-1], nil
	}).AnyTimes()
	return stream
}
