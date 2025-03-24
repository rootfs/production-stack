package validation

import (
	"fmt"
	"strconv"

	"github.com/vllm-project/production-stack/router-controller/internal/engine"
)

// ValidateEngineConfig validates engine-specific configuration
func ValidateEngineConfig(engineType engine.EngineType, config map[string]string) error {
	switch engineType {
	case engine.EngineTypeVLLM:
		return validateVLLMConfig(config)
	case engine.EngineTypeSGLang:
		return validateSGLangConfig(config)
	default:
		return fmt.Errorf("unsupported engine type: %s", engineType)
	}
}

// validateVLLMConfig validates vLLM-specific configuration
func validateVLLMConfig(config map[string]string) error {
	// Required fields
	requiredFields := []string{
		"tensor-parallel-size",
		"max-num-batched-tokens",
		"max-num-seqs",
	}

	for _, field := range requiredFields {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("required field %s is missing", field)
		}
	}

	// Validate tensor-parallel-size
	if tensorParallelSize, err := strconv.Atoi(config["tensor-parallel-size"]); err != nil {
		return fmt.Errorf("invalid tensor-parallel-size: %v", err)
	} else if tensorParallelSize <= 0 {
		return fmt.Errorf("tensor-parallel-size must be positive")
	}

	// Validate max-num-batched-tokens
	if maxNumBatchedTokens, err := strconv.Atoi(config["max-num-batched-tokens"]); err != nil {
		return fmt.Errorf("invalid max-num-batched-tokens: %v", err)
	} else if maxNumBatchedTokens <= 0 {
		return fmt.Errorf("max-num-batched-tokens must be positive")
	}

	// Validate max-num-seqs
	if maxNumSeqs, err := strconv.Atoi(config["max-num-seqs"]); err != nil {
		return fmt.Errorf("invalid max-num-seqs: %v", err)
	} else if maxNumSeqs <= 0 {
		return fmt.Errorf("max-num-seqs must be positive")
	}

	return nil
}

// validateSGLangConfig validates SGLang-specific configuration
func validateSGLangConfig(config map[string]string) error {
	// Required fields
	requiredFields := []string{
		"max-num-batched-tokens",
		"max-num-seqs",
	}

	for _, field := range requiredFields {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("required field %s is missing", field)
		}
	}

	// Validate max-num-batched-tokens
	if maxNumBatchedTokens, err := strconv.Atoi(config["max-num-batched-tokens"]); err != nil {
		return fmt.Errorf("invalid max-num-batched-tokens: %v", err)
	} else if maxNumBatchedTokens <= 0 {
		return fmt.Errorf("max-num-batched-tokens must be positive")
	}

	// Validate max-num-seqs
	if maxNumSeqs, err := strconv.Atoi(config["max-num-seqs"]); err != nil {
		return fmt.Errorf("invalid max-num-seqs: %v", err)
	} else if maxNumSeqs <= 0 {
		return fmt.Errorf("max-num-seqs must be positive")
	}

	return nil
}
