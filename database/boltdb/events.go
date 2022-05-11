package boltdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	proto "github.com/golang/protobuf/proto"
	"github.com/ohsu-comp-bio/funnel/events"
	"github.com/ohsu-comp-bio/funnel/tes"
)

// WriteEvent creates an event for the server to handle.
func (taskBolt *BoltDB) WriteEvent(ctx context.Context, req *events.Event) (*events.WriteEventResponse, error) {
	var err error

	if req.Type == events.Type_TASK_CREATED {
		task := req.GetTask()
		idBytes := []byte(task.Id)
		taskString, err := proto.Marshal(task)
		if err != nil {
			return nil, err
		}
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket(TaskBucket).Put(idBytes, taskString)
			tx.Bucket(TaskState).Put(idBytes, []byte(tes.State_QUEUED.String()))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error storing task in database: %s", err)
		}
		err = taskBolt.queueTask(task)
		if err != nil {
			return nil, fmt.Errorf("error queueing task in database: %s", err)
		}
		return &events.WriteEventResponse{}, nil
	}

	// Check that the task exists
	err = taskBolt.db.View(func(tx *bolt.Tx) error {
		_, err := getTaskView(tx, req.Id, tes.View_MINIMAL)
		return err
	})
	if err != nil {
		return nil, err
	}

	tl := &tes.TaskLog{}
	el := &tes.ExecutorLog{}

	switch req.Type {
	case events.Type_TASK_STATE:
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return transitionTaskState(tx, req.Id, req.GetState())
		})

	case events.Type_TASK_START_TIME:
		tl.StartTime = req.GetStartTime()
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateTaskLogs(tx, req.Id, tl)
		})

	case events.Type_TASK_END_TIME:
		tl.EndTime = req.GetEndTime()
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateTaskLogs(tx, req.Id, tl)
		})

	case events.Type_TASK_OUTPUTS:
		tl.Outputs = req.GetOutputs().Value
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateTaskLogs(tx, req.Id, tl)
		})

	case events.Type_TASK_METADATA:
		tl.Metadata = req.GetMetadata().Value
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateTaskLogs(tx, req.Id, tl)
		})

	case events.Type_EXECUTOR_START_TIME:
		el.StartTime = req.GetStartTime()
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateExecutorLogs(tx, fmt.Sprint(req.Id, req.Index), el)
		})

	case events.Type_EXECUTOR_END_TIME:
		el.EndTime = req.GetEndTime()
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateExecutorLogs(tx, fmt.Sprint(req.Id, req.Index), el)
		})

	case events.Type_EXECUTOR_EXIT_CODE:
		el.ExitCode = req.GetExitCode()
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateExecutorLogs(tx, fmt.Sprint(req.Id, req.Index), el)
		})

	case events.Type_EXECUTOR_STDOUT:
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateExecutorStdout(tx, fmt.Sprint(req.Id, req.Index), req.GetStdout())
		})

	case events.Type_EXECUTOR_STDERR:
		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			return updateExecutorStderr(tx, fmt.Sprint(req.Id, req.Index), req.GetStderr())
		})

	case events.Type_SYSTEM_LOG:
		var syslogs []string
		idBytes := []byte(req.Id)

		err = taskBolt.db.View(func(tx *bolt.Tx) error {
			existing := tx.Bucket(SysLogs).Get(idBytes)
			if existing != nil {
				return json.Unmarshal(existing, &syslogs)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}

		syslogs = append(syslogs, req.SysLogString())

		logbytes, err := json.Marshal(syslogs)
		if err != nil {
			return nil, err
		}

		err = taskBolt.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket(SysLogs).Put(idBytes, logbytes)
			return nil
		})
	}

	return nil, err
}

func transitionTaskState(tx *bolt.Tx, id string, target tes.State) error {
	idBytes := []byte(id)
	current := getTaskState(tx, id)

	switch {
	case target == current:
		// Current state matches target state. Do nothing.
		return nil

	case tes.TerminalState(target) && tes.TerminalState(current):
		// Avoid switching between two terminal states.
		return fmt.Errorf("Won't switch between two terminal states: %s -> %s",
			current, target)

	case tes.TerminalState(current) && !tes.TerminalState(target):
		// Error when trying to switch out of a terminal state to a non-terminal one.
		return fmt.Errorf("Unexpected transition from %s to %s", current, target)

	case target == tes.State_QUEUED:
		return fmt.Errorf("Can't transition to Queued state")
	}

	switch target {
	case tes.State_UNKNOWN, tes.State_PAUSED:
		return fmt.Errorf("Unimplemented task state %s", target)

	case tes.State_CANCELED, tes.State_COMPLETE, tes.State_EXECUTOR_ERROR, tes.State_SYSTEM_ERROR:
		// Remove from queue
		tx.Bucket(TasksQueued).Delete(idBytes)

	case tes.State_RUNNING, tes.State_INITIALIZING:
		if current != tes.State_UNKNOWN && current != tes.State_QUEUED && current != tes.State_INITIALIZING {
			return fmt.Errorf("Unexpected transition from %s to %s", current, target)
		}
		tx.Bucket(TasksQueued).Delete(idBytes)

	default:
		return fmt.Errorf("Unknown target state: %s", target)
	}

	tx.Bucket(TaskState).Put(idBytes, []byte(target.String()))
	return nil
}

func updateTaskLogs(tx *bolt.Tx, id string, tl *tes.TaskLog) error {
	tasklog := &tes.TaskLog{}

	// Try to load existing task log
	b := tx.Bucket(TasksLog).Get([]byte(id))
	if b != nil {
		err := proto.Unmarshal(b, tasklog)
		if err != nil {
			return err
		}
	}

	if tl.StartTime != "" {
		tasklog.StartTime = tl.StartTime
	}

	if tl.EndTime != "" {
		tasklog.EndTime = tl.EndTime
	}

	if tl.Outputs != nil {
		tasklog.Outputs = tl.Outputs
	}

	if tl.Metadata != nil {
		if tasklog.Metadata == nil {
			tasklog.Metadata = map[string]string{}
		}
		for k, v := range tl.Metadata {
			tasklog.Metadata[k] = v
		}
	}

	logbytes, err := proto.Marshal(tasklog)
	if err != nil {
		return err
	}
	return tx.Bucket(TasksLog).Put([]byte(id), logbytes)
}

func updateExecutorLogs(tx *bolt.Tx, id string, el *tes.ExecutorLog) error {
	// Check if there is an existing task log
	o := tx.Bucket(ExecutorLogs).Get([]byte(id))
	if o != nil {
		// There is an existing log in the DB, load it
		existing := &tes.ExecutorLog{}
		err := proto.Unmarshal(o, existing)
		if err != nil {
			return err
		}

		el.Stdout = ""
		el.Stderr = ""

		// Merge the updates into the existing.
		proto.Merge(existing, el)
		// existing is updated, so set that to el which will get saved below.
		el = existing
	}

	// Save the updated log
	logbytes, err := proto.Marshal(el)
	if err != nil {
		return err
	}
	return tx.Bucket(ExecutorLogs).Put([]byte(id), logbytes)
}

func updateExecutorStdout(tx *bolt.Tx, id, stdout string) error {
	return tx.Bucket(ExecutorStdout).Put([]byte(id), []byte(stdout))
}

func updateExecutorStderr(tx *bolt.Tx, id, stderr string) error {
	return tx.Bucket(ExecutorStderr).Put([]byte(id), []byte(stderr))
}
