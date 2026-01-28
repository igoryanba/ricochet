package config

import (
	"encoding/json"
	"testing"
)

func TestToolsSettings_JSON(t *testing.T) {
	jsonStr := `{"tools": {"disable_llm_correction": true}, "theme": "dark"}`
	var settings Settings
	if err := json.Unmarshal([]byte(jsonStr), &settings); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !settings.Tools.DisableLLMCorrection {
		t.Error("Expected DisableLLMCorrection to be true")
	}
}
