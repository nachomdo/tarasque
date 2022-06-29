package kafkabench

import (
	"errors"
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/go-resty/resty/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/jarcoal/httpmock"
)

type mockResolver struct {
	result []string
	err    error
}

func (mr *mockResolver) resolveHeadlessService() ([]string, error) {
	return mr.result, mr.err
}

func TestCollectWorkerTaskResult(t *testing.T) {
	httpClient := resty.New()
	svcResolver := &mockResolver{[]string{defaultAgentServiceName}, nil}
	client := newTrogdorServiceWithRestClient(httpClient, svcResolver)
	httpmock.ActivateNonDefault(httpClient.GetClient())
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", agentServiceURL+"/agent/status",
		func(req *http.Request) (*http.Response, error) {
			statusResponse := AgentStatusResponse{
				ServerStartMs: 1000,
				Workers: map[string]AgentStatusWorkers{
					"task-with-error-no-status": {
						State:     "DONE",
						TaskID:    "1",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status:    nil,
						Error:     "worker expired",
					},
					"task-with-status-and-error": {
						State:     "DONE",
						TaskID:    "2",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status:    "Creating 5 topic(s)",
						Error:     "Unable to create topic(s): mytopic1, mytopic2, mytopic3, mytopic4, mytopic5after 3 attempt(s)",
					},
					"task-with-results": {
						State:     "DONE",
						TaskID:    "3",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status: map[string]interface{}{
							"totalSent":             2497001,
							"averageLatencyMs":      350.56488,
							"p50LatencyMs":          16,
							"p95LatencyMs":          72,
							"p99LatencyMs":          10000,
							"transactionsCommitted": 0,
						},
					},
				},
			}
			return httpmock.NewJsonResponse(200, statusResponse)
		},
	)
	cases := map[string]struct {
		status *AgentStatusWorkers
		err    error
	}{
		"task-with-error-no-status":  {status: nil, err: errors.New("worker expired")},
		"task-with-status-and-error": {status: nil, err: errors.New("Unable to create topic(s): mytopic1, mytopic2, mytopic3, mytopic4, mytopic5after 3 attempt(s)")},
		"task-with-results": {
			status: &AgentStatusWorkers{
				State:     "DONE",
				TaskID:    "3",
				StartedMs: 1649460862398,
				DoneMs:    1649460862431,
				Status: map[string]interface{}{
					"totalSent":             float64(2497001),
					"averageLatencyMs":      350.56488,
					"p50LatencyMs":          float64(16),
					"p95LatencyMs":          float64(72),
					"p99LatencyMs":          float64(10000),
					"transactionsCommitted": float64(0),
				},
			},
		},
	}

	for input, expected := range cases {

		status, err := client.CollectWorkerTaskResult(input)

		if diff := cmp.Diff(expected.err, err, test.EquateErrors()); diff != "" {
			t.Errorf("\n%s\nclient.CollectWorkerTaskResult(...): -want error, +got error:\n%s\n", input, diff)
		}

		if diff := cmp.Diff(expected.status, status); diff != "" {
			t.Errorf("\n%s\nclient.CollectWorkerTaskResult(...): -want status, +got status:\n%s\n", input, diff)
		}

	}
}
