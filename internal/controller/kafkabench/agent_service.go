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

	"github.com/crossplane/provider-template/apis/tarasque/v1alpha1"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
)

const (
	defaultAgentServiceUrl = "https://tarasque-agent.tarasque.svc.cluster.local"
)

var (
	agentServiceUrl = getEnvOrDefault("SERVICE_URL", defaultAgentServiceUrl)
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
	wtMap["workerId"] = wt.WorkerId
	return wtMap, nil
}

//A service to communicate with the Trogdor Agent REST API
type TrogdorAgentService struct {
	client *resty.Client
}

type AgentStatusWorkers struct {
	State     string      `json:"state,omitempty"`
	TaskId    string      `json:"taskId,omitempty"`
	StartedMs int64       `json:"startedMs,omitempty"`
	DoneMs    int64       `json:"doneMs,omitempty"`
	Status    interface{} `json:"status,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type AgentStatusResponse struct {
	ServerStartMs int64                         `json:"serverStartMs,omitempty"`
	Workers       map[string]AgentStatusWorkers `json:"workers,omitempty"`
}

//Create a new instance of Trogdor Service
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

func (tas *TrogdorAgentService) CreateWorkerTask(spec v1alpha1.KafkaBenchSpec) (*WorkerTask, error) {
	payload := WorkerTask{Spec: WorkerTaskSpec{spec, time.Now().UnixMilli()}, WorkerId: rand.Int63(), TaskId: uuid.New().String()}

	body, err := sanitizeWorkerTask(&payload)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Creating: %+v \n", body)
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(body).Post(agentServiceUrl + "/agent/worker/create")

	fmt.Printf("Response: %v \n", string(resp.Body()))
	if resp.StatusCode() != http.StatusOK || err != nil {
		return nil, err
	}
	return &payload, nil
}

func (tas *TrogdorAgentService) CollectWorkerTaskResult(workerId string) (*AgentStatusWorkers, error) {
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		Get(agentServiceUrl + "/agent/status")

	if resp.StatusCode() != http.StatusOK || err != nil {
		return nil, err
	}
	agentStatusResponse := AgentStatusResponse{}
	if err := json.Unmarshal(resp.Body(), &agentStatusResponse); err != nil {
		return nil, err
	}

	workerStatus := agentStatusResponse.Workers[workerId]
	if workerStatus.Error != "" {
		return nil, errors.New(workerStatus.Error)
	}

	return &workerStatus, nil
}

func (tas *TrogdorAgentService) DeleteWorkerTask(workerId int64) {

}
