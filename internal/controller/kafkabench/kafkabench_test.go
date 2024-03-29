/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kafkabench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/jarcoal/httpmock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nachomdo/tarasque/apis/tarasque/v1alpha1"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

func TestObserve(t *testing.T) {
	type fields struct {
		service *TrogdorAgentService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalObservation
		err error
	}
	client := NewTrogdorService()

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"test": {
			"test",
			fields{service: client},
			args{
				context.TODO(),
				&v1alpha1.KafkaBench{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.KafkaBenchSpec{
						Class:            "org.apache.kafka.trogdor.workload.ProduceBenchSpec",
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
					},
				},
			},
			want{
				managed.ExternalObservation{
					ResourceExists:    false,
					ResourceUpToDate:  false,
					ConnectionDetails: managed.ConnectionDetails{},
				},
				nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
			got, err := e.Observe(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type fields struct {
		service *TrogdorAgentService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalCreation
		err error
	}

	connDetails := managed.ConnectionDetails{}

	httpClient := resty.New()
	svcResolver := &mockResolver{[]string{defaultAgentServiceName}, nil}
	client := newTrogdorServiceWithRestClient(httpClient, svcResolver)
	httpmock.ActivateNonDefault(httpClient.GetClient())
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("POST", agentServiceURL+"/agent/worker/create",
		func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			wt := WorkerTask{}
			json.Unmarshal(body, &wt)
			fmt.Printf("%v", wt)

			connDetails["taskId"] = []byte(wt.TaskID)
			connDetails["name"] = []byte("newBenchmark")
			connDetails["namespace"] = []byte("test")

			fmt.Printf("%v", connDetails)
			resp := httpmock.NewStringResponse(200, "OK")

			return resp, nil
		},
	)
	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"test": {
			"test",
			fields{service: client},

			args{
				context.TODO(),
				&v1alpha1.KafkaBench{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.KafkaBenchSpec{
						Class:            "org.apache.kafka.trogdor.workload.ProduceBenchSpec",
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
					},
				},
			},
			want{
				managed.ExternalCreation{
					ConnectionDetails: connDetails,
				},
				nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
			got, err := e.Create(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type fields struct {
		service *TrogdorAgentService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalUpdate
		err error
	}

	connDetails := managed.ConnectionDetails{}

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
					"1234": {
						State:     "DONE",
						TaskID:    "1",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status:    nil,
						Error:     "worker expired",
					},
					"1111": {
						State:     "DONE",
						TaskID:    "2",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status:    "Creating 5 topic(s)",
						Error:     "Unable to create topic(s): mytopic1, mytopic2, mytopic3, mytopic4, mytopic5after 3 attempt(s)",
					},
					"9999": {
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
					"2222": {
						State:     "DONE",
						TaskID:    "4",
						StartedMs: 1649460862398,
						DoneMs:    1649460862431,
						Status: map[string]interface{}{
							"consumer.aea227a6-b614-472d-a829-f2694766c32c-0": map[string]interface{}{
								"assignedPartitions": []string{
									"test3-8",
									"test1-6",
									"test4-8",
								},
								"totalMessagesReceived":   1500,
								"totalBytesReceived":      774000,
								"averageMessageSizeBytes": 516,
								"averageLatencyMs":        2036.3334,
								"p50LatencyMs":            1,
								"p95LatencyMs":            6108,
								"p99LatencyMs":            6108,
							},
						},
					},
				},
			}
			return httpmock.NewJsonResponse(200, statusResponse)
		},
	)
	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"producerBench": {
			"producerBenchTest",
			fields{service: client},

			args{
				context.TODO(),
				&v1alpha1.KafkaBench{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.KafkaBenchSpec{
						Class:            producerWorkload,
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
					},
					Status: v1alpha1.KafkaBenchStatus{
						AtProvider: v1alpha1.KafkaBenchObservation{
							WorkerID:   9999,
							TaskID:     "3",
							TaskStatus: "CREATED",
						},
					},
				},
			},
			want{
				managed.ExternalUpdate{
					ConnectionDetails: connDetails,
				},
				nil,
			},
		},
		"consumerBenchTest": {
			"consumerBenchTest",
			fields{service: client},
			args{
				context.TODO(),
				&v1alpha1.KafkaBench{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.KafkaBenchSpec{
						Class:            consumerWorkload,
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
					},
					Status: v1alpha1.KafkaBenchStatus{
						AtProvider: v1alpha1.KafkaBenchObservation{
							WorkerID:   2222,
							TaskID:     "4",
							TaskStatus: "CREATED",
						},
					},
				},
			},
			want{
				managed.ExternalUpdate{
					ConnectionDetails: connDetails,
				},
				nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
			got, err := e.Update(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Update(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Update(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
