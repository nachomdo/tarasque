package kafkabench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/nachomdo/tarasque/apis/tarasque/v1alpha1"
)

const (
	defaultAgentServiceName = "tarasque-agent.tarasque.svc.cluster.local"
	defaultAgentServiceURL  = "http://tarasque-agent.tarasque.svc.cluster.local"
	defaultAgentServicePort = "8888"
)

var (
	agentServiceURL  = getEnvOrDefault("SERVICE_URL", defaultAgentServiceURL)
	agentServicePort = getEnvOrDefault("SERVICE_PORT", defaultAgentServicePort)
	sanitizeFields   = []string{"providerConfigRef", "forProvider", "deletionPolicy"}
)

type resolver interface {
	resolveHeadlessService() ([]string, error)
}

type kubeResolver struct{}

func (*kubeResolver) resolveHeadlessService() ([]string, error) {
	_, addrs, err := net.LookupSRV("", "", defaultAgentServiceName)

	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, errors.New("not available workers registered in the Agent Service")
	}
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, net.JoinHostPort(strings.TrimSuffix(addr.Target, "."), agentServicePort))
	}

	return result, nil
}

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
	client      *resty.Client
	svcResolver resolver
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
		client:      resty.New(),
		svcResolver: &kubeResolver{},
	}
}

func newTrogdorServiceWithRestClient(httpClient *resty.Client, svcResolver resolver) *TrogdorAgentService {
	return &TrogdorAgentService{
		client:      httpClient,
		svcResolver: svcResolver,
	}
}

// CreateWorkerTask initiates a new worker task on Trogdor agents
func (tas *TrogdorAgentService) CreateWorkerTask(spec v1alpha1.KafkaBenchSpec) (*WorkerTask, error) {
	//nolint
	payload := WorkerTask{Spec: WorkerTaskSpec{spec, time.Now().UnixMilli()}, WorkerID: rand.Int63(), TaskID: uuid.New().String()}

	body, err := sanitizeWorkerTask(&payload)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Creating: %+v \n", body)
	addrs, err := tas.svcResolver.resolveHeadlessService()
	if err != nil {
		return nil, err
	}
	g, _ := errgroup.WithContext(context.Background())

	for _, addr := range addrs {
		endpoint := addr
		g.Go(func() error {
			resp, err := tas.client.NewRequest().
				SetHeader("Accept", "application/json").
				SetHeader("Content-Type", "application/json").
				SetBody(body).Post(fmt.Sprintf("http://%s/agent/worker/create", endpoint))

			if err != nil {
				return err
			}
			fmt.Printf("Response: %v \n", string(resp.Body()))
			if resp.StatusCode() != http.StatusOK || err != nil {
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return &payload, err
	}
	return &payload, nil
}

// CollectWorkerTaskResult checks the status of a given workerID in Trogdor agents
func (tas *TrogdorAgentService) CollectWorkerTaskResult(workerID string) (*AgentStatusWorkers, error) {
	addrs, err := tas.svcResolver.resolveHeadlessService()
	if err != nil || len(addrs) == 0 {
		return nil, errors.New("non resolvable address returned")
	}
	//nolint
	idx := rand.Int() % len(addrs)
	resp, err := tas.client.NewRequest().
		SetHeader("Accept", "application/json").
		Get(fmt.Sprintf("http://%s/agent/status", addrs[idx]))

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
	addrs, err := tas.svcResolver.resolveHeadlessService()
	if err != nil {
		return errors.New("non resolvable address returned")
	}
	for _, addr := range addrs {
		_, err = tas.client.NewRequest().
			SetHeader("Accept", "application/json").
			Delete(fmt.Sprintf("http://%s/agent/worker?workerId=%s", addr, workerID))

		if err != nil {
			return err
		}
	}

	return nil
}
