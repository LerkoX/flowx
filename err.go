package pipelinex

import "errors"

var (
	ErrInvalidGraph = errors.New("invalid graph")
	ErrHasCycle     = errors.New("has cycle")

	// 动态修改相关错误
	ErrPipelineRunning    = errors.New("pipeline is running, cannot modify")
	ErrNodeNotFound       = errors.New("node not found in graph")
	ErrEdgeNotFound       = errors.New("edge not found in graph")
	ErrNodeAlreadyRunning = errors.New("node is currently running, cannot remove")
	ErrInvalidState       = errors.New("invalid state for this operation")
)
