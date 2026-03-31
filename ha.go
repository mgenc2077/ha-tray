package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// toggleEntity toggles the state of a given Home Assistant entity using the REST API.
func toggleEntity(entityID string) error {

	url := fmt.Sprintf("%s/api/services/homeassistant/toggle", config.HaURL)

	payload := map[string]string{
		"entity_id": entityID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.HaToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// toggleEntityWs toggles the state of a given Home Assistant entity using the WebSocket API.
func toggleEntityWs(entityID string) error {

	wsURL := strings.Replace(config.HaURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	if !strings.HasSuffix(wsURL, "/") {
		wsURL += "/"
	}
	wsURL += "api/websocket"

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial error: %w", err)
	}
	defer c.Close()

	var msg map[string]interface{}
	err = c.ReadJSON(&msg)
	if err != nil {
		return fmt.Errorf("failed to read auth_required message: %w", err)
	}
	if msg["type"] != "auth_required" {
		return fmt.Errorf("expected auth_required, got: %v", msg["type"])
	}

	authMsg := map[string]interface{}{
		"type":         "auth",
		"access_token": config.HaToken,
	}
	err = c.WriteJSON(authMsg)
	if err != nil {
		return fmt.Errorf("failed to send auth message: %w", err)
	}

	err = c.ReadJSON(&msg)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}
	if msg["type"] != "auth_ok" {
		return fmt.Errorf("authentication failed: %v", msg)
	}

	callServiceMsg := map[string]interface{}{
		"id":      1,
		"type":    "call_service",
		"domain":  "homeassistant",
		"service": "toggle",
		"service_data": map[string]interface{}{
			"entity_id": entityID,
		},
	}
	err = c.WriteJSON(callServiceMsg)
	if err != nil {
		return fmt.Errorf("failed to send call_service message: %w", err)
	}

	err = c.ReadJSON(&msg)
	if err != nil {
		return fmt.Errorf("failed to read call_service result: %w", err)
	}
	if msg["type"] != "result" || msg["success"] != true {
		return fmt.Errorf("call_service failed: %v", msg)
	}

	return nil
}

// discovery fetches all available entities from Home Assistant and logs them.
func discovery() error {
	url := fmt.Sprintf("%s/api/states", config.HaURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.HaToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var states []struct {
		EntityID string `json:"entity_id"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal(body, &states); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	log.Println("--- Home Assistant Entities ---")
	for _, s := range states {
		log.Printf("Entity: %s | State: %s", s.EntityID, s.State)
	}
	log.Printf("Total entities discovered: %d", len(states))
	log.Println("-------------------------------")

	return nil
}
