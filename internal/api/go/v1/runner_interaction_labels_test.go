package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWatchRequestType(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Type string
	}

	tests := []struct {
		Name     string
		Request  any
		Expected Expectation
	}{
		{
			Name:    "send_message",
			Request: &WatchRequestsResponse_CallSendMessageToAgentExecution{},
			Expected: Expectation{
				Type: "call_send_message_to_agent_execution",
			},
		},
		{
			Name:    "runner_configuration_change",
			Request: &WatchRequestsResponse_EventRunnerConfigurationChange{},
			Expected: Expectation{
				Type: "event_runner_configuration_change",
			},
		},
		{
			Name:    "unknown",
			Request: nil,
			Expected: Expectation{
				Type: "unknown",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{
				Type: WatchRequestType(tc.Request),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("WatchRequestType() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
