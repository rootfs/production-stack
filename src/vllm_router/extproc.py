import argparse
import asyncio
import json
import logging
import os
import sys
import time
import uuid
from concurrent import futures
from typing import AsyncIterable, Dict, List, Optional, Tuple

import grpc
from fastapi import Request
from google.protobuf.struct_pb2 import Struct
from grpc.aio import ServicerContext

# Import Envoy ExtProc proto-generated classes
from envoy.service.ext_proc.v3.external_processor_pb2 import ProcessingRequest, ProcessingResponse
from envoy.service.ext_proc.v3.external_processor_pb2_grpc import ExternalProcessorServicer, add_ExternalProcessorServicer_to_server
from envoy.config.core.v3.base_pb2 import HeaderMap, HeaderValue
from envoy.type.v3.http_status_pb2 import StatusCode

# Import VLLM router components
from vllm_router.batch import BatchProcessor, initialize_batch_processor
from vllm_router.dynamic_config import (
    DynamicRouterConfig,
    GetDynamicConfigWatcher,
    InitializeDynamicConfigWatcher,
)
from vllm_router.engine_stats import GetEngineStatsScraper, InitializeEngineStatsScraper
from vllm_router.experimental.feature_gates import (
    get_feature_gates,
    initialize_feature_gates,
)
from vllm_router.experimental.semantic_cache import (
    GetSemanticCache,
    InitializeSemanticCache,
    enable_semantic_cache,
    is_semantic_cache_enabled,
)
from vllm_router.experimental.semantic_cache_integration import (
    add_semantic_cache_args,
    check_semantic_cache,
    semantic_cache_hit_ratio,
    semantic_cache_hits,
    semantic_cache_latency,
    semantic_cache_misses,
    semantic_cache_size,
    store_in_semantic_cache,
)
from vllm_router.files import Storage, initialize_storage
from vllm_router.httpx_client import HTTPXClientWrapper
from vllm_router.log import init_logger
from vllm_router.request_stats import (
    GetRequestStatsMonitor,
    InitializeRequestStatsMonitor,
)
from vllm_router.routing_logic import GetRoutingLogic, InitializeRoutingLogic
from vllm_router.service_discovery import (
    GetServiceDiscovery,
    InitializeServiceDiscovery,
    ServiceDiscoveryType,
)
from vllm_router.utils import (
    parse_static_model_names,
    parse_static_urls,
    set_ulimit,
    validate_url,
)

logger = init_logger(__name__)
httpx_client_wrapper = HTTPXClientWrapper()

class VLLMExtProcServicer(ExternalProcessorServicer):
    """
    Implements the Envoy ExtProc gRPC service interface for VLLM routing.
    This service processes HTTP requests and responses, routing them to the appropriate
    VLLM backend based on the model specified in the request and route configuration.
    """
    
    def __init__(self):
        """Initialize the ExtProc servicer."""
        self.request_contexts = {}
        self.engine_stats_scraper = GetEngineStatsScraper()
        self.request_stats_monitor = GetRequestStatsMonitor()
        self.router = GetRoutingLogic()
        self.route_configs = {}  # Maps route names to their configurations
        httpx_client_wrapper.start()
    
    def add_route(self, route_name: str, model_name: str, path_prefix: str = None):
        """
        Add a route configuration.
        
        Args:
            route_name: The name of the route.
            model_name: The name of the model to use for this route.
            path_prefix: Optional path prefix to match for this route.
        """
        self.route_configs[route_name] = {
            "model": model_name,
            "path_prefix": path_prefix
        }
    
    async def Process(self, request_iterator: AsyncIterable[ProcessingRequest], 
                     context: ServicerContext) -> AsyncIterable[ProcessingResponse]:
        """
        Process a stream of ProcessingRequest messages from Envoy.
        
        Args:
            request_iterator: An async iterator of ProcessingRequest messages.
            context: The gRPC service context.
            
        Yields:
            ProcessingResponse messages to send back to Envoy.
        """
        # Generate a unique ID for this request
        request_id = str(uuid.uuid4())
        self.request_contexts[request_id] = {
            "headers": {},
            "body": bytearray(),
            "method": None,
            "path": None,
            "model": None,
            "route": None,
            "backend_url": None,
            "start_time": time.time(),
            "response_started": False,
            "response_body": bytearray(),
            "response_headers": {},
            "response_status_code": 200,
        }
        
        try:
            async for request in request_iterator:
                response = await self._handle_processing_request(request, request_id)
                yield response
        except Exception as e:
            logger.error(f"Error processing request {request_id}: {str(e)}")
            yield self._create_immediate_response(
                status_code=500,
                body=json.dumps({"error": f"Internal server error: {str(e)}"}).encode(),
                headers={"content-type": "application/json"}
            )
        finally:
            if request_id in self.request_contexts:
                del self.request_contexts[request_id]
    
    async def _handle_processing_request(self, request: ProcessingRequest, 
                                        request_id: str) -> ProcessingResponse:
        """
        Handle a single ProcessingRequest message.
        
        Args:
            request: The ProcessingRequest message to handle.
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        # Check which type of request we received
        if request.HasField("request_headers"):
            return await self._handle_request_headers(request.request_headers, request_id)
        elif request.HasField("request_body"):
            return await self._handle_request_body(request.request_body, request_id)
        elif request.HasField("response_headers"):
            return await self._handle_response_headers(request.response_headers, request_id)
        elif request.HasField("response_body"):
            return await self._handle_response_body(request.response_body, request_id)
        else:
            # For other message types, just continue processing
            return self._create_continue_response()
    
    async def _handle_request_headers(self, headers_msg, request_id: str) -> ProcessingResponse:
        """
        Handle request headers from Envoy.
        
        Args:
            headers_msg: The request headers message.
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        context = self.request_contexts[request_id]
        
        # Extract headers
        for header in headers_msg.headers.headers:
            name = header.key.lower()
            value = header.value
            context["headers"][name] = value
            
            # Extract method and path
            if name == ":method":
                context["method"] = value
            elif name == ":path":
                context["path"] = value
            # Extract route name from Envoy metadata if available
            elif name == "x-envoy-route-name":
                context["route"] = value
        
        # If we have a route configuration, use it to determine the model
        if context["route"] and context["route"] in self.route_configs:
            route_config = self.route_configs[context["route"]]
            context["model"] = route_config["model"]
            
            # Check path prefix if configured
            if route_config["path_prefix"] and not context["path"].startswith(route_config["path_prefix"]):
                return self._create_immediate_response(
                    status_code=404,
                    body=json.dumps({"error": "Path does not match route configuration"}).encode(),
                    headers={"content-type": "application/json"}
                )
            
            # If we have the model from route config, we can route immediately
            return await self._route_request(request_id)
        
        # If no route match and path starts with /v1/, try to extract model from body
        if context["path"] and context["path"].startswith("/v1/"):
            return self._create_continue_response(body_mode="BUFFERED")
        
        # For non-API paths without route match, just continue without modification
        return self._create_continue_response()
    
    async def _handle_request_body(self, body_msg, request_id: str) -> ProcessingResponse:
        """
        Handle request body from Envoy.
        
        Args:
            body_msg: The request body message.
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        context = self.request_contexts[request_id]
        
        # If we already have a model from route config, just continue
        if context["model"]:
            return self._create_continue_response()
        
        # Append body data
        if body_msg.body:
            context["body"].extend(body_msg.body)
        
        # If this is the end of the body, process it
        if body_msg.end_of_stream:
            try:
                # Parse the body as JSON to extract the model
                if context["body"]:
                    body_json = json.loads(context["body"])
                    context["model"] = body_json.get("model")
                
                # If we have a model, route the request
                if context["model"]:
                    return await self._route_request(request_id)
                else:
                    return self._create_immediate_response(
                        status_code=400,
                        body=json.dumps({"error": "Invalid request: missing 'model' in request body."}).encode(),
                        headers={"content-type": "application/json"}
                    )
            except json.JSONDecodeError:
                return self._create_immediate_response(
                    status_code=400,
                    body=json.dumps({"error": "Invalid JSON in request body."}).encode(),
                    headers={"content-type": "application/json"}
                )
        
        return self._create_continue_response()
    
    async def _route_request(self, request_id: str) -> ProcessingResponse:
        """
        Route the request to the appropriate backend based on the model.
        
        Args:
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        context = self.request_contexts[request_id]
        model = context["model"]
        
        # Get endpoints for the requested model
        endpoints = GetServiceDiscovery().get_endpoint_info()
        engine_stats = self.engine_stats_scraper.get_engine_stats()
        request_stats = self.request_stats_monitor.get_request_stats(time.time())
        
        # Filter endpoints for the requested model
        endpoints = list(filter(lambda x: x.model_name == model, endpoints))
        if not endpoints:
            return self._create_immediate_response(
                status_code=400,
                body=json.dumps({"error": f"Model {model} not found."}).encode(),
                headers={"content-type": "application/json"}
            )
        
        # Create a mock request object for the router
        mock_request = MockRequest(
            method=context["method"],
            headers=context["headers"],
            body=bytes(context["body"]),
            path=context["path"]
        )
        
        # Use the routing logic to determine the backend URL
        backend_url = self.router.route_request(
            endpoints, engine_stats, request_stats, mock_request
        )
        
        logger.info(f"Routing request {request_id} for model {model} to {backend_url}")
        context["backend_url"] = backend_url
        
        # Record the start of the request
        self.request_stats_monitor.on_new_request(backend_url, request_id, time.time())
        
        # Continue processing, we'll handle the actual forwarding in the response phase
        return self._create_continue_response()
    
    async def _handle_response_headers(self, headers_msg, request_id: str) -> ProcessingResponse:
        """
        Handle response headers from Envoy.
        
        Args:
            headers_msg: The response headers message.
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        context = self.request_contexts[request_id]
        
        # Extract headers
        for header in headers_msg.headers.headers:
            name = header.key.lower()
            value = header.value
            context["response_headers"][name] = value
            
            # Extract status code
            if name == ":status":
                try:
                    context["response_status_code"] = int(value)
                except ValueError:
                    pass
        
        # If we have a backend URL, we need to forward the request
        if context["backend_url"] and not context["response_started"]:
            # Request the response body so we can modify it
            return self._create_continue_response(body_mode="BUFFERED")
        
        # Otherwise, continue without modification
        return self._create_continue_response()
    
    async def _handle_response_body(self, body_msg, request_id: str) -> ProcessingResponse:
        """
        Handle response body from Envoy.
        
        Args:
            body_msg: The response body message.
            request_id: The unique ID for this request.
            
        Returns:
            A ProcessingResponse message to send back to Envoy.
        """
        context = self.request_contexts[request_id]
        
        # Append body data
        if body_msg.body:
            context["response_body"].extend(body_msg.body)
        
        # If this is the end of the body and we have a backend URL, forward the request
        if body_msg.end_of_stream and context["backend_url"] and not context["response_started"]:
            context["response_started"] = True
            
            try:
                # Forward the request to the backend
                client = httpx_client_wrapper()
                response = await client.request(
                    method=context["method"],
                    url=context["backend_url"] + context["path"],
                    headers={k: v for k, v in context["headers"].items() if not k.startswith(":")},
                    content=bytes(context["body"]),
                    timeout=None,
                )
                
                # Record the response
                self.request_stats_monitor.on_request_response(
                    context["backend_url"], request_id, time.time()
                )
                
                # Create response headers
                response_headers = {
                    ":status": str(response.status_code),
                    "content-type": response.headers.get("content-type", "application/json"),
                }
                
                # Add any other headers from the response
                for name, value in response.headers.items():
                    if name.lower() not in [":status", "content-type"]:
                        response_headers[name.lower()] = value
                
                # Create the immediate response
                return self._create_immediate_response(
                    status_code=response.status_code,
                    body=response.content,
                    headers=response_headers
                )
            except Exception as e:
                logger.error(f"Error forwarding request {request_id}: {str(e)}")
                return self._create_immediate_response(
                    status_code=500,
                    body=json.dumps({"error": f"Error forwarding request: {str(e)}"}).encode(),
                    headers={"content-type": "application/json"}
                )
            finally:
                # Record the completion of the request
                self.request_stats_monitor.on_request_complete(
                    context["backend_url"], request_id, time.time()
                )
        
        # If not end of stream or no backend URL, continue processing
        return self._create_continue_response()
    
    def _create_continue_response(self, body_mode: Optional[str] = None) -> ProcessingResponse:
        """
        Create a ProcessingResponse that tells Envoy to continue processing.
        
        Args:
            body_mode: Optional processing mode for the body.
            
        Returns:
            A ProcessingResponse message.
        """
        response = ProcessingResponse()
        
        # If a body mode is specified, set the processing mode
        if body_mode:
            response.mode_override.request_body_mode = getattr(
                response.mode_override, body_mode
            )
        
        return response
    
    def _create_immediate_response(self, status_code: int, body: bytes, 
                                  headers: Dict[str, str]) -> ProcessingResponse:
        """
        Create a ProcessingResponse that sends an immediate response to the client.
        
        Args:
            status_code: The HTTP status code to send.
            body: The response body to send.
            headers: The response headers to send.
            
        Returns:
            A ProcessingResponse message.
        """
        response = ProcessingResponse()
        
        # Set the immediate response
        immediate_response = response.immediate_response
        immediate_response.status.code = status_code
        
        # Set the body if provided
        if body:
            immediate_response.body = body
        
        # Set the headers
        for name, value in headers.items():
            if not name.startswith(":"):
                header = immediate_response.headers.headers.add()
                header.key = name
                header.value = value
        
        return response


class MockRequest:
    """
    A mock Request object that mimics the FastAPI Request object.
    This is used to provide compatibility with the existing VLLM router.
    """
    
    def __init__(self, method: str, headers: Dict[str, str], body: bytes, path: str):
        """
        Initialize a mock request.
        
        Args:
            method: The HTTP method.
            headers: The request headers.
            body: The request body.
            path: The request path.
        """
        self.method = method
        self._headers = headers
        self._body = body
        self.url = MockURL(path)
    
    @property
    def headers(self):
        """Get the request headers."""
        return self._headers
    
    async def body(self):
        """Get the request body."""
        return self._body
    
    async def json(self):
        """Parse the request body as JSON."""
        return json.loads(self._body)


class MockURL:
    """A mock URL object that mimics the FastAPI URL object."""
    
    def __init__(self, path: str):
        """
        Initialize a mock URL.
        
        Args:
            path: The request path.
        """
        self.path = path


async def serve_async(port: int = 50051):
    """
    Start the gRPC server asynchronously.
    
    Args:
        port: The port to listen on.
    """
    server = grpc.aio.server(futures.ThreadPoolExecutor(max_workers=10))
    add_ExternalProcessorServicer_to_server(VLLMExtProcServicer(), server)
    server.add_insecure_port(f'[::]:{port}')
    
    await server.start()
    logger.info(f"VLLM ExtProc server started on port {port}")
    
    try:
        await server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down server...")
        await server.stop(5)  # 5 seconds grace period


def serve(port: int = 50051):
    """
    Start the gRPC server.
    
    Args:
        port: The port to listen on.
    """
    asyncio.run(serve_async(port))


def parse_args():
    """Parse command line arguments."""
    parser = argparse.ArgumentParser(description="VLLM Router ExtProc Service")
    
    # Add the same arguments as before
    parser.add_argument("--host", type=str, default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=50051, help="Port to bind to")
    
    # Service discovery arguments
    parser.add_argument(
        "--service-discovery",
        type=str,
        choices=["static", "k8s"],
        default="static",
        help="Service discovery type",
    )
    parser.add_argument(
        "--static-backends",
        type=str,
        default="",
        help="Comma-separated list of backend URLs (for static service discovery)",
    )
    parser.add_argument(
        "--static-models",
        type=str,
        default="",
        help="Comma-separated list of model names (for static service discovery)",
    )
    
    # Route configuration arguments
    parser.add_argument(
        "--route-config",
        type=str,
        default=None,
        help="Path to JSON file containing route configurations",
    )
    
    # Routing logic arguments
    parser.add_argument(
        "--routing-logic",
        type=str,
        choices=["round_robin", "least_outstanding_requests", "session_aware"],
        default="least_outstanding_requests",
        help="Routing logic to use",
    )
    parser.add_argument(
        "--session-key",
        type=str,
        default="user_id",
        help="Session key to use for session-aware routing",
    )
    
    # Engine stats arguments
    parser.add_argument(
        "--engine-stats-interval",
        type=int,
        default=30,
        help="The interval in seconds to scrape engine statistics.",
    )
    parser.add_argument(
        "--request-stats-window",
        type=int,
        default=60,
        help="The sliding window in seconds to compute request statistics.",
    )
    
    # Batch API arguments
    parser.add_argument(
        "--enable-batch-api",
        action="store_true",
        help="Enable the batch API",
    )
    parser.add_argument(
        "--file-storage-class",
        type=str,
        default="local",
        choices=["local", "s3"],
        help="File storage class to use for batch API",
    )
    parser.add_argument(
        "--file-storage-path",
        type=str,
        default="/tmp/vllm-router",
        help="File storage path to use for batch API",
    )
    parser.add_argument(
        "--batch-processor",
        type=str,
        default="local",
        choices=["local"],
        help="Batch processor to use for batch API",
    )
    
    # Dynamic config arguments
    parser.add_argument(
        "--dynamic-config-json",
        type=str,
        default=None,
        help="The path to the json file containing the dynamic configuration.",
    )
    
    # Feature gates arguments
    parser.add_argument(
        "--feature-gates",
        type=str,
        default="",
        help="Comma-separated list of feature gates (e.g., 'SemanticCache=true')",
    )
    
    # Add semantic cache arguments
    add_semantic_cache_args(parser)
    
    # Log level argument
    parser.add_argument(
        "--log-level",
        type=str,
        default="info",
        choices=["critical", "error", "warning", "info", "debug", "trace"],
        help="Log level. Default is 'info'.",
    )
    
    return parser.parse_args()


def load_route_config(config_path: str) -> List[Dict]:
    """
    Load route configurations from a JSON file.
    
    Args:
        config_path: Path to the JSON configuration file.
        
    Returns:
        List of route configurations.
        
    Example config format:
    [
        {
            "name": "chat-route",
            "model": "gpt-3.5-turbo",
            "path_prefix": "/v1/chat"
        },
        {
            "name": "completion-route",
            "model": "text-davinci-003",
            "path_prefix": "/v1/completions"
        }
    ]
    """
    try:
        with open(config_path) as f:
            return json.load(f)
    except Exception as e:
        logger.error(f"Error loading route config from {config_path}: {str(e)}")
        return []


def initialize_all(args):
    """
    Initialize all the components of the router with the given arguments.
    
    Args:
        args: The parsed command-line arguments.
    """
    if args.service_discovery == "static":
        InitializeServiceDiscovery(
            ServiceDiscoveryType.STATIC,
            urls=parse_static_urls(args.static_backends),
            models=parse_static_model_names(args.static_models),
        )
    elif args.service_discovery == "k8s":
        InitializeServiceDiscovery(
            ServiceDiscoveryType.K8S,
            namespace=args.k8s_namespace,
            port=args.k8s_port,
            label_selector=args.k8s_label_selector,
        )
    else:
        raise ValueError(f"Invalid service discovery type: {args.service_discovery}")
    
    # Initialize singletons
    InitializeEngineStatsScraper(args.engine_stats_interval)
    InitializeRequestStatsMonitor(args.request_stats_window)
    
    if args.enable_batch_api:
        logger.info("Initializing batch API")
        initialize_storage(args.file_storage_class, args.file_storage_path)
        initialize_batch_processor(args.batch_processor, args.file_storage_path, None)
    
    InitializeRoutingLogic(args.routing_logic, session_key=args.session_key)
    
    # Initialize feature gates
    initialize_feature_gates(args.feature_gates)
    
    # Check if the SemanticCache feature gate is enabled
    feature_gates = get_feature_gates()
    if feature_gates.is_enabled("SemanticCache"):
        # The feature gate is enabled, explicitly enable the semantic cache
        enable_semantic_cache()
        
        # Verify that the semantic cache was successfully enabled
        if not is_semantic_cache_enabled():
            logger.error("Failed to enable semantic cache feature")
        
        logger.info("SemanticCache feature gate is enabled")
        
        # Initialize the semantic cache with the model if specified
        if args.semantic_cache_model:
            logger.info(f"Initializing semantic cache with model: {args.semantic_cache_model}")
            logger.info(f"Semantic cache directory: {args.semantic_cache_dir or 'default'}")
            logger.info(f"Semantic cache threshold: {args.semantic_cache_threshold}")
            
            cache = InitializeSemanticCache(
                embedding_model=args.semantic_cache_model,
                cache_dir=args.semantic_cache_dir,
                default_similarity_threshold=args.semantic_cache_threshold,
            )
            
            # Update cache size metric
            if cache and hasattr(cache, "db") and hasattr(cache.db, "index"):
                semantic_cache_size.labels(server="router").set(cache.db.index.ntotal)
                logger.info(f"Semantic cache initialized with {cache.db.index.ntotal} entries")
            
            logger.info(f"Semantic cache initialized with model {args.semantic_cache_model}")
        else:
            logger.warning(
                "SemanticCache feature gate is enabled but no embedding model specified. "
                "The semantic cache will not be functional without an embedding model. "
                "Use --semantic-cache-model to specify an embedding model."
            )
    elif args.semantic_cache_model:
        logger.warning(
            "Semantic cache model specified but SemanticCache feature gate is not enabled. "
            "Enable the feature gate with --feature-gates=SemanticCache=true"
        )
    
    # Initialize dynamic config watcher
    if args.dynamic_config_json:
        init_config = DynamicRouterConfig.from_args(args)
        InitializeDynamicConfigWatcher(args.dynamic_config_json, 10, init_config, None)


def main():
    """Main entry point."""
    args = parse_args()
    
    # Configure logging
    logging_level = getattr(logging, args.log_level.upper())
    logging.basicConfig(level=logging_level)
    
    # Initialize all components
    initialize_all(args)
    
    # Set ulimit to avoid issues with too many open files
    set_ulimit()
    
    # Create the ExtProc servicer
    servicer = VLLMExtProcServicer()
    
    # Load route configurations if provided
    if args.route_config:
        route_configs = load_route_config(args.route_config)
        for route in route_configs:
            servicer.add_route(
                route_name=route["name"],
                model_name=route["model"],
                path_prefix=route.get("path_prefix")
            )
            logger.info(f"Added route {route['name']} for model {route['model']}")
    
    # Start the gRPC server
    logger.info(f"Starting VLLM ExtProc server on port {args.port}")
    
    # Create and start the server with our servicer
    server = grpc.aio.server(futures.ThreadPoolExecutor(max_workers=10))
    add_ExternalProcessorServicer_to_server(servicer, server)
    server.add_insecure_port(f'[::]:{args.port}')
    
    # Run the server
    asyncio.run(serve_async(args.port))


if __name__ == "__main__":
    main() 