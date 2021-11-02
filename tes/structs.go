package tes

import (
	"github.com/ohsu-comp-bio/funnel/tes/openapi"
)

/*
These are wrapper structers that map data structures named after the
old protobuf code to the new structures generated by the OpenAPI generator
*/

type ServiceInfoRequest struct {
}

type GetTaskRequest struct {
	Id   string   `json:"id"`
	View TaskView `json:"view"`
}

type CancelTaskRequest struct {
	Id string `json:"id"`
}

type ListTasksRequest struct {
	NamePrefix string   `json:"namePrefix"`
	PageSize   int64    `json:"pageSize"`
	PageToken  string   `json:"pageToken"`
	View       TaskView `json:"view"`
}

type Task = openapi.TesTask

type ListTasksResponse = openapi.TesListTasksResponse

type CreateTaskResponse = openapi.TesCreateTaskResponse

type CancelTaskResponse = openapi.ImplResponse

type ServiceInfo = openapi.Service

type TaskLog = openapi.TesTaskLog

type ExecutorLog = openapi.TesExecutorLog

type State = openapi.TesState

type Resources = openapi.TesResources

type Executor = openapi.TesExecutor

const (
	State_UNKNOWN        State = "UNKNOWN"
	State_QUEUED         State = "QUEUED"
	State_INITIALIZING   State = "INITIALIZING"
	State_RUNNING        State = "RUNNING"
	State_PAUSED         State = "PAUSED"
	State_COMPLETE       State = "COMPLETE"
	State_EXECUTOR_ERROR State = "EXECUTOR_ERROR"
	State_SYSTEM_ERROR   State = "SYSTEM_ERROR"
	State_CANCELED       State = "CANCELED"
)

type TaskView string

const (
	TaskView_MINIMAL TaskView = "MINIMAL"
	TaskView_BASIC   TaskView = "BASIC"
	TaskView_FULL    TaskView = "FULL"
)

type FileType = openapi.TesFileType

const (
	FileType_FILE      FileType = "FILE"
	FileType_DIRECTORY FileType = "DIRECTORY"
)

type Input = openapi.TesInput

type Output = openapi.TesOutput

type OutputFileLog = openapi.TesOutputFileLog
