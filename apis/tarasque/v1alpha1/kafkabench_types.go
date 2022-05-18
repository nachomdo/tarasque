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

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// KafkaBenchObservation are the observable fields of a KafkaBench.
type KafkaBenchObservation struct {
	TaskStatus     string                              `json:"taskStatus,omitempty"`
	TaskId         string                              `json:"taskId,omitempty"`
	WorkerId       int64                               `json:"workerId,omitempty"`
	ProducerStats  ProducerBenchResultStats            `json:"producerStats,omitempty"`
	ConsumerStats  map[string]ConsumerBenchResultStats `json:"consumerStats,omitempty"`
	RoundTripStats RoundTripBenchResultStats           `json:"roundTripStats,omitempty"`
}

// KafkaTopics are part of the desired state fields
type KafkaTopics struct {
	NumPartitions     int16 `json:"numPartitions,omitempty"`
	ReplicationFactor int8  `json:"replicationFactor,omitempty"`
}

// A KafkaBenchSpec defines the desired state of a KafkaBench.
type KafkaBenchSpec struct {
	xpv1.ResourceSpec    `json:",inline"`
	Class                string                 `json:"class,omitempty"`
	DurationMs           int64                  `json:"durationMs,omitempty"`
	ProducerNode         string                 `json:"producerNode,omitempty"`
	ConsumerNode         string                 `json:"consumerNode,omitempty"`
	ClientNode           string                 `json:"clientNode,omitempty"`
	ConsumerGroup        string                 `json:"consumerGroup,omitempty"`
	ThreadsPerWorker     int32                  `json:"threadsPerWorker,omitempty"`
	BootstrapServers     string                 `json:"bootstrapServers,omitempty"`
	TargetMessagesPerSec int32                  `json:"targetMessagesPerSec,omitempty"`
	MaxMessages          int64                  `json:"maxMessages,omitempty"`
	ActiveTopics         map[string]KafkaTopics `json:"activeTopics,omitempty"`
	InactiveTopics       map[string]KafkaTopics `json:"inactiveTopics,omitempty"`
	ProducerConf         map[string]string      `json:"producerConf,omitempty"`
	ConsumerConf         map[string]string      `json:"consumerConf,omitempty"`
	CommonClientConf     map[string]string      `json:"commonClientConf,omitempty"`
	AdminClientConf      map[string]string      `json:"adminClientConf,omitempty"`
}

// A ProducerBenchResultStats represents the benchmarking results obtained by the agent
type ProducerBenchResultStats struct {
	TotalSent             int64   `json:"totalSent,omitempty"`
	AverageLatencyMs      float64 `json:"averageLatencyMs,omitempty"`
	P50LatencyMs          int64   `json:"p50LatencyMs,omitempty"`
	P95LatencyMs          int64   `json:"p95LatencyMs,omitempty"`
	P99LatencyMs          int64   `json:"p99LatencyMs,omitempty"`
	TransactionsCommitted int64   `json:"transactionsCommitted,omitempty"`
}

// A RoundTripBenchResultStats represents the benchmarking results obtained by the agent
type RoundTripBenchResultStats struct {
	TotalUniqueSent int64 `json:"totalUniqueSent,omitempty"`
	TotalReceived   int64 `json:"totalReceived,omitempty"`
}

// A ConsumerBenchResultStats represents the benchmarking results obtained by the agent
type ConsumerBenchResultStats struct {
	AssignedPartitions      []string          `json:"assignedPartitions,omitempty"`
	TotalMessagesReceived   int64             `json:"totalMessagesReceived,omitempty"`
	TotalBytesReceived      int64             `json:"totalBytesReceived,omitempty"`
	AverageMessageSizeBytes int64             `json:"averageMessageSizeBytes,omitempty"`
	AverageLatencyMs        float64           `json:"averageLatencyMs,omitempty"`
	P50LatencyMs            int64             `json:"p50LatencyMs,omitempty"`
	P95LatencyMs            int64             `json:"p95LatencyMs,omitempty"`
	P99LatencyMs            int64             `json:"p99LatencyMs,omitempty"`
	RecordProcessorStatus   map[string]string `json:"recordProcessorStatus,omitempty"`
}

// A KafkaBenchStatus represents the observed state of a KafkaBench.
type KafkaBenchStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KafkaBenchObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KafkaBench is an tarasque API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,template}
type KafkaBench struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaBenchSpec   `json:"spec"`
	Status KafkaBenchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaBenchList contains a list of KafkaBench
type KafkaBenchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaBench `json:"items"`
}

// KafkaBench type metadata.
var (
	KafkaBenchKind             = reflect.TypeOf(KafkaBench{}).Name()
	KafkaBenchGroupKind        = schema.GroupKind{Group: Group, Kind: KafkaBenchKind}.String()
	KafkaBenchKindAPIVersion   = KafkaBenchKind + "." + SchemeGroupVersion.String()
	KafkaBenchGroupVersionKind = SchemeGroupVersion.WithKind(KafkaBenchKind)
)

func init() {
	SchemeBuilder.Register(&KafkaBench{}, &KafkaBenchList{})
}
