import argparse

from vllm_router.version import __version__

try:
    # Semantic cache integration
    from vllm_router.experimental.semantic_cache import (
        GetSemanticCache,
        enable_semantic_cache,
        initialize_semantic_cache,
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

    semantic_cache_available = True
except ImportError:
    semantic_cache_available = False


# --- Argument Parsing and Initialization ---
def validate_args(args):
    if args.service_discovery == "static":
        if args.static_backends is None:
            raise ValueError(
                "Static backends must be provided when using static service discovery."
            )
        if args.static_models is None:
            raise ValueError(
                "Static models must be provided when using static service discovery."
            )
    if args.service_discovery == "k8s" and args.k8s_port is None:
        raise ValueError("K8s port must be provided when using K8s service discovery.")
    if args.routing_logic == "session" and args.session_key is None:
        raise ValueError(
            "Session key must be provided when using session routing logic."
        )
    if args.log_stats and args.log_stats_interval <= 0:
        raise ValueError("Log stats interval must be greater than 0.")
    if args.engine_stats_interval <= 0:
        raise ValueError("Engine stats interval must be greater than 0.")
    if args.request_stats_window <= 0:
        raise ValueError("Request stats window must be greater than 0.")


def parse_args():
    """Parse command line arguments."""
    parser = argparse.ArgumentParser(
        description="vLLM router for load balancing and request routing."
    )

    # Service discovery arguments
    parser.add_argument(
        "--service-discovery",
        type=str,
        choices=["static", "k8s"],
        default="static",
        help="Service discovery method to use.",
    )
    parser.add_argument(
        "--static-backends",
        type=str,
        default="http://localhost:8000",
        help="Comma-separated list of backend URLs (for static service discovery).",
    )
    parser.add_argument(
        "--static-models",
        type=str,
        default="default",
        help="Comma-separated list of model names (for static service discovery).",
    )
    parser.add_argument(
        "--k8s-namespace",
        type=str,
        help="Kubernetes namespace to watch for vLLM pods.",
    )
    parser.add_argument(
        "--k8s-port",
        type=int,
        default=8000,
        help="Port number for vLLM pods in Kubernetes.",
    )
    parser.add_argument(
        "--k8s-label-selector",
        type=str,
        help="Label selector for vLLM pods in Kubernetes.",
    )

    # Routing arguments
    parser.add_argument(
        "--routing-logic",
        type=str,
        choices=["roundrobin", "session"],
        default="roundrobin",
        help="Routing logic to use.",
    )
    parser.add_argument(
        "--session-key",
        type=str,
        default="session_id",
        help="Header key for session-based routing.",
    )

    # Server arguments
    parser.add_argument(
        "--host",
        type=str,
        default="0.0.0.0",
        help="Host to bind the server to.",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8001,
        help="Port to bind the server to.",
    )

    # Batch API arguments
    parser.add_argument(
        "--enable-batch-api",
        action="store_true",
        help="Enable batch API endpoints.",
    )
    parser.add_argument(
        "--file-storage-class",
        type=str,
        default="local",
        help="Storage class for file storage.",
    )
    parser.add_argument(
        "--file-storage-path",
        type=str,
        default="/tmp/vllm-router",
        help="Path for file storage.",
    )
    parser.add_argument(
        "--batch-processor",
        type=str,
        default="local",
        help="Batch processor type.",
    )

    # Stats arguments
    parser.add_argument(
        "--engine-stats-interval",
        type=int,
        default=10,
        help="Interval in seconds to scrape engine stats.",
    )
    parser.add_argument(
        "--request-stats-window",
        type=int,
        default=60,
        help="Window size in seconds for request stats.",
    )
    parser.add_argument(
        "--log-stats",
        action="store_true",
        help="Enable logging of statistics.",
    )
    parser.add_argument(
        "--log-stats-interval",
        type=int,
        default=10,
        help="The interval in seconds to log statistics.",
    )

    # Feature gates
    parser.add_argument(
        "--feature-gates",
        type=str,
        help="Comma-separated list of feature gates (e.g. SemanticCache=true,PIIDetection=true).",
    )

    # PII detection arguments
    parser.add_argument(
        "--pii-analyzer",
        type=str,
        choices=["presidio", "regex"],
        default="presidio",
        help="PII analyzer to use.",
    )
    parser.add_argument(
        "--pii-languages",
        type=str,
        default="en",
        help="Comma-separated list of languages for PII detection.",
    )
    parser.add_argument(
        "--pii-score-threshold",
        type=float,
        default=0.5,
        help="Score threshold for PII detection (0.0-1.0).",
    )

    # Add --version argument
    parser.add_argument(
        "--version",
        action="store_true",
        help="Show version information.",
    )

    if semantic_cache_available:
        add_semantic_cache_args(parser)

    # Add log level argument
    parser.add_argument(
        "--log-level",
        type=str,
        default="info",
        choices=["critical", "error", "warning", "info", "debug", "trace"],
        help="Log level for uvicorn. Default is 'info'.",
    )

    # Add dynamic config argument
    parser.add_argument(
        "--dynamic-config-json",
        type=str,
        help="Path to dynamic configuration JSON file",
    )

    args = parser.parse_args()
    validate_args(args)
    return args
