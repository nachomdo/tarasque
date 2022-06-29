![image](https://user-images.githubusercontent.com/3109377/176461760-13a1ea19-bfd0-4fc9-b9d4-32fa6a994434.png)

# Tarasque

A Cloud Native Kafka benchmarking tool using [Trogdor](https://github.com/apache/kafka/blob/trunk/TROGDOR.md).

## Pre-requisites

* Kubectl 
* Helm
* Crossplane plugin for Kubectl
* Kafka 

## 10.000ft overview

![image](https://user-images.githubusercontent.com/3109377/176470511-fb63bcee-d934-44fc-8e84-3ef0f6cdb54e.png)

Tarasque removes the friction of running [Trogdor](https://github.com/apache/kafka/blob/trunk/TROGDOR.md) on Kubernetes. It packages and deploys Trogdor Agents as Kubernetes DaemonSets and implements a Crossplane-based operator (provider) to watch and act on KafkaBench configurations. The KafkaBench CRD is just a direct YAML translations from [Trogdor API requests](https://github.com/apache/kafka/tree/trunk/tests/spec). Trogdor Coordinator has been replaced with the Tarasque Controller which creates and tracks new and existing tasks in all the available workers. 

As an alternative, Trogdor Agents can be installed as a Kubernetes Deployment and they can be scaled, manually or automatically, to the number of instances necessaries to max out your Kafka cluster.

## Quick start guide 

1. Create a Kubernetes cluster either local or in your favourite Cloud provider. This guide will use GKE AutoPilot to spin up a production ready cluster. 

```bash
$ gcloud container clusters create-auto tarasque-gke-autopilot --region europe-west2
```

2. Clone this repository and run `make install` 

3. Create a Confluent Cloud cluster 

```bash
$ confluent kafka cluster create --cloud gcp --region europe-west2 --type basic --availability single-zone --environment <<your-environment-here>> tarasque-test

It may take up to 5 minutes for the Kafka cluster to be ready.
+---------------+---------------+
| ID            | lkc-xxxxxx    |
| Name          | tarasque-test |
| Type          | BASIC         |
| Ingress       |           100 |
| Egress        |           100 |
| Storage       | 5 TB          |
| Provider      | gcp           |
| Availability  | single-zone   |
| Region        | europe-west2  |
| Status        | PROVISIONING  |
| Endpoint      |               |
| API Endpoint  |               |
| REST Endpoint |               |
+---------------+---------------+
```

4. As an alternative, you can deploy your Kafka cluster in Kubernetes using [CFK](https://docs.confluent.io/operator/current/co-quickstart.html)

```bash
$ helm repo add confluentinc https://packages.confluent.io/helm
$ helm repo update

$ helm upgrade --install --create-namespace --namespace confluent confluent-operator confluentinc/confluent-for-kubernetes

$ kubectl apply -f https://raw.githubusercontent.com/confluentinc/confluent-kubernetes-examples/master/quickstart-deploy/confluent-platform-singlenode.yaml
```

5. Deploy a Trogdor based configuration to benchmark your cluster. Find more benchmark configuration examples [here](./examples/sample) 

```bash
$ kubectl apply -n tarasque -f - <<EOF
apiVersion: tarasque.crossplane.io/v1alpha1
kind: KafkaBench
metadata:
  name: producer-benchmark
spec:
  class: org.apache.kafka.trogdor.workload.ProduceBenchSpec
  durationMs: 10000000
  producerNode: node0
  bootstrapServers: pkc-xxxx.europe-west1.gcp.confluent.cloud:9092
  commonClientConf:
    security.protocol: SASL_SSL
    sasl.jaas.config: org.apache.kafka.common.security.plain.PlainLoginModule  required username='XXXXXX' password='XXXXX';
    sasl.mechanism: PLAIN
  targetMessagesPerSec: 10000
  maxMessages: 150000
  activeTopics:
    test[1-5]:
      numPartitions: 10
      replicationFactor: 3
  providerConfigRef:
    name: example
EOF
```

6. Check the status of your KafkaBench. Benchmark results will be appended to the status subresource when tasks are done. 

```bash
$ kubectl -n tarasque get KafkaBench producer-benchmark -o yaml

apiVersion: tarasque.crossplane.io/v1alpha1
kind: KafkaBench
metadata:
  annotations:
    crossplane.io/external-name: producer-benchmark
  name: producer-benchmark
[... redacted ...]  
status:
  atProvider:
    producerStats: {}
    roundTripStats: {}
    taskId: 957c447f-d215-4449-a784-ba8164460613
    taskStatus: RUNNING
    workerId: 7369853788303479649
```

7. Check Confluent Cloud UI for your cluster

8. To remove Tarasque from your cluster just run `make uninstall` 