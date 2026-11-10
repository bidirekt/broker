package repository

import (
	"encoding/json"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
)

func toContractContent(files []model.ContractFile) string {
	content, err := json.Marshal(files)
	if err != nil {
		panic(fmt.Errorf("error encoding contract content: %w", err))
	}

	return string(content)
}

func toContractFiles(content string) []model.ContractFile {
	var files []model.ContractFile
	if err := json.Unmarshal([]byte(content), &files); err != nil {
		panic(fmt.Errorf("error decoding contract content: %w", err))
	}

	return files
}
