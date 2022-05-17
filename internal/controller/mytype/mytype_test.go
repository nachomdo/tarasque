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

package mytype

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/provider-template/apis/sample/v1alpha1"
	"github.com/jarcoal/httpmock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				&v1alpha1.MyType{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.MyTypeSpec{
						Class:            "org.apache.kafka.trogdor.workload.ProduceBenchSpec",
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
						ForProvider: v1alpha1.MyTypeParameters{
							ConfigurableField: "example",
						},
					},
				},
			},
			want{
				managed.ExternalObservation{
					ResourceExists:    false,
					ResourceUpToDate:  true,
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
	client := newTrogdorServiceWithRestClient(httpClient)
	httpmock.ActivateNonDefault(httpClient.GetClient())
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("POST", agentServiceUrl+"/agent/worker/create",
		func(req *http.Request) (*http.Response, error) {
			body, _ := ioutil.ReadAll(req.Body)
			wt := WorkerTask{}
			json.Unmarshal(body, &wt)
			fmt.Printf("%v", wt)

			connDetails["taskId"] = []byte(wt.TaskId)
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
				&v1alpha1.MyType{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "newBenchmark",
					},
					Spec: v1alpha1.MyTypeSpec{
						Class:            "org.apache.kafka.trogdor.workload.ProduceBenchSpec",
						BootstrapServers: "localhost:9092",
						ActiveTopics: map[string]v1alpha1.KafkaTopics{
							"myTopic": {
								NumPartitions:     10,
								ReplicationFactor: 3,
							},
						},
						ForProvider: v1alpha1.MyTypeParameters{
							ConfigurableField: "example",
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
