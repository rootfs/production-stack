# Router Controller for Production Stack

This controller manages routing and traffic control for inference services in the Production Stack. It provides sophisticated routing capabilities, speculative decoding support, and caching mechanisms for large language model inference.

## Architecture

### Components

1. **Router Controller**
   - Manages custom resources for inference routing
   - Handles GPU-aware service discovery and load balancing
   - Implements prefix-based routing for cache efficiency
   - Manages session affinity for multi-turn conversations

2. **Custom Resources**
   - `InferenceService`: Defines model serving endpoints and configurations
   - `InferenceGateway`: Manages GPU-aware routing rules and service discovery
   - `InferenceCache`: Handles KV caching configurations for inference
   - `SpeculativeDecoding`: Configures speculative decoding for LLMs
   - `PrefillDecodingDisaggregation`: Manages PDD configuration and topology

### System Requirements

- Kubernetes
- kubectl
- NVIDIA GPU metrics exporter (for GPU-aware routing)

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
