package tools

import (
	"encoding/json"
	"fmt"
)

// parseTool unmarshals a tool call's raw JSON input into the given type.
func parseTool[T any](raw string) (*T, error) {
	var result T
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parsing tool input: %w", err)
	}
	return &result, nil
}

// ParsePlanSkeleton parses a tool call's raw JSON input into PlanSkeletonInput.
func ParsePlanSkeleton(raw string) (*PlanSkeletonInput, error) { return parseTool[PlanSkeletonInput](raw) }

// ParseWriteCode parses a tool call's raw JSON input into WriteCodeInput.
func ParseWriteCode(raw string) (*WriteCodeInput, error) { return parseTool[WriteCodeInput](raw) }

// ParseReportFix parses a tool call's raw JSON input into ReportFixInput.
func ParseReportFix(raw string) (*ReportFixInput, error) { return parseTool[ReportFixInput](raw) }
