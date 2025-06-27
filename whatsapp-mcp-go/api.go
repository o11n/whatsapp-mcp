package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SendMessageRequest represents the request body for the send message API
type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	MediaPath string `json:"media_path,omitempty"`
}

// SendMessageResponse represents the response for the send message API
type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DownloadMediaRequest represents the request body for the download media API
type DownloadMediaRequest struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
}

// DownloadMediaResponse represents the response for the download media API
type DownloadMediaResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

func sendMessage(recipient, message string) (bool, string) {
	if recipient == "" {
		return false, "Recipient must be provided"
	}

	url := fmt.Sprintf("%s/send", WHATSAPP_API_BASE_URL)
	payload := SendMessageRequest{
		Recipient: recipient,
		Message:   message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("JSON marshal error: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Sprintf("Request error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("Error reading response: %v", err)
	}

	if resp.StatusCode == 200 {
		var result SendMessageResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return false, fmt.Sprintf("Error parsing response: %s", string(body))
		}
		return result.Success, result.Message
	} else {
		return false, fmt.Sprintf("Error: HTTP %d - %s", resp.StatusCode, string(body))
	}
}

func sendFile(recipient, mediaPath string) (bool, string) {
	if recipient == "" {
		return false, "Recipient must be provided"
	}

	if mediaPath == "" {
		return false, "Media path must be provided"
	}

	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return false, fmt.Sprintf("Media file not found: %s", mediaPath)
	}

	url := fmt.Sprintf("%s/send", WHATSAPP_API_BASE_URL)
	payload := SendMessageRequest{
		Recipient: recipient,
		MediaPath: mediaPath,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("JSON marshal error: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Sprintf("Request error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("Error reading response: %v", err)
	}

	if resp.StatusCode == 200 {
		var result SendMessageResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return false, fmt.Sprintf("Error parsing response: %s", string(body))
		}
		return result.Success, result.Message
	} else {
		return false, fmt.Sprintf("Error: HTTP %d - %s", resp.StatusCode, string(body))
	}
}

func sendAudioMessage(recipient, mediaPath string) (bool, string) {
	if recipient == "" {
		return false, "Recipient must be provided"
	}

	if mediaPath == "" {
		return false, "Media path must be provided"
	}

	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return false, fmt.Sprintf("Media file not found: %s", mediaPath)
	}

	// Convert to opus ogg if not already
	if !strings.HasSuffix(mediaPath, ".ogg") {
		convertedPath, err := convertToOpusOggTemp(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error converting file to opus ogg. You likely need to install ffmpeg: %v", err)
		}
		mediaPath = convertedPath
	}

	url := fmt.Sprintf("%s/send", WHATSAPP_API_BASE_URL)
	payload := SendMessageRequest{
		Recipient: recipient,
		MediaPath: mediaPath,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("JSON marshal error: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Sprintf("Request error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("Error reading response: %v", err)
	}

	if resp.StatusCode == 200 {
		var result SendMessageResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return false, fmt.Sprintf("Error parsing response: %s", string(body))
		}
		return result.Success, result.Message
	} else {
		return false, fmt.Sprintf("Error: HTTP %d - %s", resp.StatusCode, string(body))
	}
}

func downloadMedia(messageID, chatJID string) string {
	url := fmt.Sprintf("%s/download", WHATSAPP_API_BASE_URL)
	payload := DownloadMediaRequest{
		MessageID: messageID,
		ChatJID:   chatJID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("JSON marshal error: %v\n", err)
		return ""
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Request error: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return ""
	}

	if resp.StatusCode == 200 {
		var result DownloadMediaResponse
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("Error parsing response: %s\n", string(body))
			return ""
		}
		if result.Success {
			fmt.Printf("Media downloaded successfully: %s\n", result.Path)
			return result.Path
		} else {
			fmt.Printf("Download failed: %s\n", result.Message)
			return ""
		}
	} else {
		fmt.Printf("Error: HTTP %d - %s\n", resp.StatusCode, string(body))
		return ""
	}
}

// Audio conversion functions
func convertToOpusOgg(inputFile, outputFile string, bitrate string, sampleRate int) (string, error) {
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputFile)
	}

	if outputFile == "" {
		ext := filepath.Ext(inputFile)
		outputFile = strings.TrimSuffix(inputFile, ext) + ".ogg"
	}

	// Ensure the output directory exists
	outputDir := filepath.Dir(outputFile)
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// Build the ffmpeg command
	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-c:a", "libopus",
		"-b:a", bitrate,
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-application", "voip",
		"-vbr", "on",
		"-compression_level", "10",
		"-frame_duration", "60",
		"-y",
		outputFile,
	)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to convert audio. You likely need to install ffmpeg. Error: %v, Output: %s", err, string(output))
	}

	return outputFile, nil
}

func convertToOpusOggTemp(inputFile string) (string, error) {
	// Create a temporary file with .ogg extension
	tempFile, err := os.CreateTemp("", "audio_*.ogg")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	tempFile.Close()

	// Convert the audio
	outputFile, err := convertToOpusOgg(inputFile, tempFile.Name(), "32k", 24000)
	if err != nil {
		// Clean up the temporary file if conversion fails
		os.Remove(tempFile.Name())
		return "", err
	}

	return outputFile, nil
}
