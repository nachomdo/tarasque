package kafkabench

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"

	"github.com/nachomdo/tarasque/apis/tarasque/v1alpha1"
)

const (
	defaultAgentServiceURL = "https://tarasque-agent.tarasque.svc.cluster.local"
)

var (
	agentServiceURL = getEnvOrDefault("SERVICE_URL", defaultAgentServiceURL)
	sanitizeFields  = []string{"providerConfigRef", "forProvider", "deletionPolicy"}
)

func getEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func sanitizeWorkerTask(wt *WorkerTask) (map[string]interface{}, error) {
	jsonMap, err := json.Marshal(wt)
	if err != nil {
		return nil, err
	}

	wtMap := make(map[string]interface{}, reflect.ValueOf(*wt).NumField())
	if err := json.Unmarshal(jsonMap, &wtMap); err != nil {
		return nil, err
	}

	wtSpec := wtMap["spec"].(map[string]interface{})
	for _, field := range sanitizeFields {
		delete(wtSpec, field)
	}

	if wt.Spec.Class == consumerWorkload {
		keys := make([]string, 0, len(wt.Spec.ActiveTopics))
		for k := range wt.Spec.ActiveTopics {
			if k != "" {
				keys = append(keys, k)
			}
		}
		wtSpec["activeTopics"] = keys
	}
	wtMap["workerId"] = wt.WorkerID
	return wtMap, nil
}

// TrogdorAgentService provides access to the Trogdor Agent REST API
type TrogdorAgentService struct {
	client *resty.Client
}

// AgentStatusWorkers represents the worker status as returned by Trogdor Agent API
type AgentStatusWorkers struct {
	State     string      `json:"state,omitempty"`
	TaskID    string      `json:"taskId,omitempty"`
	StartedMs int64       `json:"startedMs,omitempty"`
	DoneMs    int64       `json:"doneMs,omitempty"`
	Status    interface{} `json:"status,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// AgentStatusResponse encapsulates the response from the Trogdor Agent status endpoint
type AgentStatusResponse struct {
	ServerStartMs int64                         `json:"serverStartMs,omitempty"`
	Workers       map[string]AgentStatusWorkers `json:"workers,omitempty"`
}

// NewTrogdorService returns a new instance of Trogdor Service
func NewTrogdorService() *TrogdorAgentService {
	return &TrogdorAgentService{
		client: resty.New(),
	}
}

func newTrogdorServiceWithRestClient(httpClient *resty.Client) *TrogdorAgentService {
	return &TrogdorAgentService{
		client: httpClient,
	}
}

// CreateWorkerTask initiates a new worker task on Trogdor agents
func (tas *TrogdorAgentService) CreateWorkerTask(spec v1alpha1.KafkaBenchSpec) (*WorkerTask, error) {
	payload := WorkerTask{Spec: WorkerTaskSpec{spec, time.Now().UnixMilli()}, WorkerID: rand.Int63(), TaskID: uuid.New().String()}

	body, err := sanitizeWorkerTask(&payload)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Creating: %+v \n", body)
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(body).Post(agentServiceURL + "/agent/worker/create")

	fmt.Printf("Response: %v \n", string(resp.Body()))
	if resp.StatusCode() != http.StatusOK || err != nil {
		return nil, err
	}
	return &payload, nil
}

// CollectWorkerTaskResult checks the status of a given workerID in Trogdor agents
func (tas *TrogdorAgentService) CollectWorkerTaskResult(workerID string) (*AgentStatusWorkers, error) {
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		Get(agentServiceURL + "/agent/status")

	if resp.StatusCode() != http.StatusOK || err != nil {
		return nil, err
	}
	agentStatusResponse := AgentStatusResponse{}
	if err := json.Unmarshal(resp.Body(), &agentStatusResponse); err != nil {
		return nil, err
	}

	workerStatus := agentStatusResponse.Workers[workerID]
	if workerStatus.Error != "" {
		return nil, errors.New(workerStatus.Error)
	}

	return &workerStatus, nil
}

// DeleteWorkerTask removes a given worker in Trogdor agents
func (tas *TrogdorAgentService) DeleteWorkerTask(workerID string) error {
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		Delete(fmt.Sprintf("%s/agent/worker?workerId=%s", agentServiceURL, workerID))

	if resp.StatusCode() != http.StatusOK || err != nil {
		return err
	}

	return nil
}
