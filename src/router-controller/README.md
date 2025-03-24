# Router Controller

The Router Controller is a Kubernetes operator that manages the deployment and configuration of large language model (LLM) inference services. It supports multiple inference engines and provides features like prefill-decoding disaggregation and speculative decoding.

## Features

- **Multiple Inference Engine Support**
  - vLLM
  - SGLang
  - Extensible architecture for adding more engines

- **Prefill-Decoding Disaggregation**
  - Separates prefill and decoding stages for better resource utilization
  - Configurable resource allocation for each stage
  - Support for different inference engines

- **Speculative Decoding**
  - Optimizes inference performance using speculative execution
  - Configurable draft model settings

- **Dynamic Service Discovery**
  - Static service discovery
  - Kubernetes service discovery
  - Custom service discovery providers

## Architecture

The Router Controller consists of several key components:

1. **Custom Resource Definitions (CRDs)**
   - `InferenceService`: The main resource that defines an inference service. It can reference other resources like PDD and SD for advanced features.
   - `PrefillDecodingDisaggregation`: Configures disaggregated prefill/decoding stages for better resource utilization.
   - `SpeculativeDecoding`: Manages speculative decoding settings for performance optimization.
   - `InferenceGateway`: Manages routing and load balancing across multiple inference services.

2. **Engine Interface**
   - Abstract interface for different inference engines
   - Engine-specific implementations for vLLM and SGLang
   - Extensible design for adding new engines

3. **Controllers**
   - `InferenceServiceReconciler`: Handles service deployments and coordinates with other features
   - `PrefillDecodingDisaggregationReconciler`: Manages disaggregated deployments
   - `SpeculativeDecodingReconciler`: Handles speculative decoding
   - `InferenceGatewayReconciler`: Manages gateway configurations

## Configuration

### InferenceService with PrefillDecodingDisaggregation

The `InferenceService` is the main resource that defines your inference service. It can optionally reference a `PrefillDecodingDisaggregation` resource to enable disaggregated prefill/decoding:

```yaml
apiVersion: production-stack.vllm.ai/v1alpha1
kind: InferenceService
metadata:
  name: gpt4-inference
spec:
  modelName: "gpt-4"
  backendRef:
    name: "gpt4-backend"
    port: 8000
  prefillDecodingRef:
    name: "example-pdd"
    apiGroup: "production-stack.vllm.ai"
    kind: "PrefillDecodingDisaggregation"
---
apiVersion: production-stack.vllm.ai/v1alpha1
kind: PrefillDecodingDisaggregation
metadata:
  name: example-pdd
spec:
  modelName: "gpt-4"
  engine: "vllm"  # or "sglang"
  engineConfig:
    tensor-parallel-size: "4"
    max-num-batched-tokens: "8192"
    max-num-seqs: "256"
  topologyHint:
    nodeSelector:
      kubernetes.io/arch: "amd64"
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: "nvidia.com/gpu"
              operator: "Exists"
  prefillResources:
    requests:
      nvidia.com/gpu: "4"
      memory: "64Gi"
    limits:
      nvidia.com/gpu: "4"
      memory: "64Gi"
  decodeResources:
    requests:
      nvidia.com/gpu: "4"
      memory: "64Gi"
    limits:
      nvidia.com/gpu: "4"
      memory: "64Gi"
```

### Engine-Specific Configuration

#### vLLM Configuration
Required fields:
- `tensor-parallel-size`: Number of GPUs for tensor parallelism
- `max-num-batched-tokens`: Maximum number of tokens in a batch
- `max-num-seqs`: Maximum number of sequences

#### SGLang Configuration
Required fields:
- `max-num-batched-tokens`: Maximum number of tokens in a batch
- `max-num-seqs`: Maximum number of sequences

## Development

### Building
```bash
make build
```

### Testing
```bash
make test
```

### Running Locally
```bash
make run
```


## Installation

### Install Router Controller

```bash
# Apply CRDs
kubectl apply -f config/crd/bases/

# Install RBAC
kubectl apply -f config/rbac/

# Deploy controller
kubectl apply -f config/manager/
```

## Usage Examples

### Basic Inference Service

```yaml
apiVersion: production-stack.vllm.ai/v1alpha1
kind: InferenceService
metadata:
  name: basic-inference
spec:
  modelName: "meta-llama/Llama-2-70b-chat-hf"
  backendRef: "llama2-service-backend"
  inferenceCacheRef: "llama2-cache"
  nlpFilters:
    semanticCache:
      storeRef: "redis-nlp-cache"
      threshold: 0.85
      ttlSeconds: 3600
```

### GPU-Aware Gateway Configuration

```yaml
apiVersion: production-stack.vllm.ai/v1alpha1
kind: InferenceGateway
metadata:
  name: llm-gateway
spec:
  schedulingPolicy: "gpu-load-aware"
  routeStrategy: "PrefixHash"
  sessionAffinity: true
  routes:
    - model: "llama3-70b"
      inferenceServiceRef: "llama3-chatbot-service"
    - model: "mistral3"
      inferenceServiceRef: "mistral-agent-service"
```

### Cache Configuration with PDD and SD

```yaml
apiVersion: production-stack.vllm.ai/v1alpha1
kind: InferenceCache
metadata:
  name: llama3-chatbot-cache
spec:
  modelName: "llama3-70b"
  kvCacheTransferPolicy:
    thresholdHitRate: 0.8
    evictionThreshold: 0.9
  pddRef: "pdd-llama3-chatbot"
  sdRef: "sd-llama3-chatbot"
---
apiVersion: production-stack.vllm.ai/v1alpha1
kind: SpeculativeDecoding
metadata:
  name: sd-llama3-chatbot
spec:
  draftModel: "llama3-1b"
  targetModel: "llama3-70b"
---
apiVersion: production-stack.vllm.ai/v1alpha1
kind: PrefillDecodingDisaggregation
metadata:
  name: pdd-llama3-chatbot
spec:
  modelName: "llama3-70b"
  topologyHint:
    nodeSelector:
      gpuType: "NVIDIA-A100"
      zone: "rack1"
```

## Configuration

### Environment Variables

- `KUBERNETES_NAMESPACE`: Namespace where the controller runs
- `METRICS_ADDR`: Address to expose metrics on
- `HEALTH_PROBE_ADDR`: Address for health probes
- `LEADER_ELECTION_ENABLED`: Enable/disable leader election
- `GPU_METRICS_ENDPOINT`: Endpoint for GPU metrics collection

## Monitoring

The controller exposes metrics at `/metrics` endpoint:

- Controller reconciliation metrics
- Custom resource status metrics
- GPU utilization metrics
- Cache performance metrics
- Routing efficiency metrics

## StaticRoute CRD

The StaticRoute CRD allows you to configure static backends and models when dynamic service discovery is not needed.

### Example

```yaml
apiVersion: production-stack.vllm.ai/v1alpha1
kind: StaticRoute
metadata:
  name: staticroute-sample
spec:
  # Service discovery method
  serviceDiscovery: static

  # Routing logic
  routingLogic: roundrobin

  # Comma-separated list of backend URLs
  staticBackends: "http://localhost:9001,http://localhost:9002,http://localhost:9003"

  # Comma-separated list of model names
  staticModels: "facebook/opt-125m,meta-llama/Llama-3.1-8B-Instruct,facebook/opt-125m"

  # Name of the router service to configure
  routerRef:
    kind: Service
    apiVersion: v1
    name: vllm-router
    namespace: default

  # Optional: Name of the ConfigMap to create
  configMapName: vllm-router-config
```

### How it works

- The controller watches for StaticRoute resources
- When a StaticRoute is created or updated, the controller creates or updates a ConfigMap with the configuration
- The ConfigMap contains a `static_config.json` file with the routing configuration
- The controller verifies the health of the configured backends

The StaticRoute resource has the following status fields:

- `configMapRef`: The name of the ConfigMap that was created
- `lastAppliedTime`: The time when the configuration was last applied
- `conditions`: A list of conditions that represent the latest available observations of the StaticRoute's state
