package whisper

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transcriber handles audio conversion and transcription
type Transcriber struct {
	whisperPath string
	modelPath   string
	tmpDir      string
}

// NewTranscriber creates a new transcriber
func NewTranscriber(whisperPath, modelPath string) (*Transcriber, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	tmpDir := filepath.Join(homeDir, ".ricochet", "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, err
	}

	return &Transcriber{
		whisperPath: whisperPath,
		modelPath:   modelPath,
		tmpDir:      tmpDir,
	}, nil
}

// Transcribe converts OGG to WAV and transcribes it
func (t *Transcriber) Transcribe(oggPath string) (string, error) {
	// 1. Convert to WAV (16kHz, mono)
	wavPath := filepath.Join(t.tmpDir, strings.TrimSuffix(filepath.Base(oggPath), filepath.Ext(oggPath))+".wav")

	cmd := exec.Command("ffmpeg", "-y", "-i", oggPath, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w (output: %s)", err, string(output))
	}
	defer os.Remove(wavPath)

	// 2. Run Whisper
	// -nt: no timestamps
	// -l auto: auto detect language
	cmd = exec.Command(t.whisperPath, "-m", t.modelPath, "-f", wavPath, "-nt", "-l", "auto")
	output, err := cmd.Output() // Use Output() instead of CombinedOutput() to ignore logs on stderr
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("whisper transcription failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("whisper transcription failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "whisper_") || strings.HasPrefix(trimmed, "system_info") || strings.HasPrefix(trimmed, "main:") {
			continue
		}
		result = append(result, trimmed)
	}

	text := strings.Join(result, " ")
	log.Printf("Transcribed text: %s", text)
	return text, nil
}
