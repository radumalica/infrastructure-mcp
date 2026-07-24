package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

// psLine mirrors the subset of `docker ps --format '{{json .}}'` fields
// (one JSON object per line, not a JSON array) that Ps needs.
type psLine struct {
	ID         string `json:"ID"`
	Image      string `json:"Image"`
	Command    string `json:"Command"`
	CreatedAt  string `json:"CreatedAt"`
	Status     string `json:"Status"`
	State      string `json:"State"`
	Ports      string `json:"Ports"`
	Names      string `json:"Names"`
	RunningFor string `json:"RunningFor"`
}

// parsePs parses the newline-delimited JSON produced by
// `docker ps --format '{{json .}}'`.
func parsePs(output string) ([]Container, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []Container
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var pl psLine
		if err := json.Unmarshal([]byte(line), &pl); err != nil {
			return nil, fmt.Errorf("docker: parse ps: %w", err)
		}
		result = append(result, Container(pl))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("docker: parse ps: %w", err)
	}
	return result, nil
}

// imageLine mirrors the subset of `docker images --format '{{json .}}'`
// fields Images needs.
type imageLine struct {
	ID         string `json:"ID"`
	Repository string `json:"Repository"`
	Tag        string `json:"Tag"`
	CreatedAt  string `json:"CreatedAt"`
	Size       string `json:"Size"`
}

// parseImages parses the newline-delimited JSON produced by
// `docker images --format '{{json .}}'`.
func parseImages(output string) ([]Image, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []Image
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var il imageLine
		if err := json.Unmarshal([]byte(line), &il); err != nil {
			return nil, fmt.Errorf("docker: parse images: %w", err)
		}
		result = append(result, Image(il))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("docker: parse images: %w", err)
	}
	return result, nil
}

// statsLine mirrors the subset of
// `docker stats --no-stream --format '{{json .}}'` fields Stats needs.
type statsLine struct {
	ID       string `json:"ID"`
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
	PIDs     string `json:"PIDs"`
}

// parseStats parses the newline-delimited JSON produced by
// `docker stats --no-stream --format '{{json .}}'`.
func parseStats(output string) ([]ContainerStats, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var result []ContainerStats
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var sl statsLine
		if err := json.Unmarshal([]byte(line), &sl); err != nil {
			return nil, fmt.Errorf("docker: parse stats: %w", err)
		}
		result = append(result, ContainerStats{
			ID:         sl.ID,
			Name:       sl.Name,
			CPUPercent: sl.CPUPerc,
			MemUsage:   sl.MemUsage,
			MemPercent: sl.MemPerc,
			NetIO:      sl.NetIO,
			BlockIO:    sl.BlockIO,
			PIDs:       sl.PIDs,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("docker: parse stats: %w", err)
	}
	return result, nil
}
