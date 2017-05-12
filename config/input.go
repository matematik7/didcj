package config

import "encoding/json"

type Input struct {
	Name            string          `json:"name"`
	Inputs          []string        `json:"inputs"`
	ReturnType      string          `json:"return_type"`
	ReturnGenerator string          `json:"return_generator"`
	ReturnConfig    json.RawMessage `json:"return_config"`
}
