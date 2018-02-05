package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/smartcontractkit/chainlink/logger"
	"github.com/tidwall/gjson"
	null "gopkg.in/guregu/null.v3"
)

type JobRun struct {
	ID        string    `json:"id" storm:"id,index,unique"`
	JobID     string    `json:"jobId" storm:"index"`
	Status    string    `json:"status" storm:"index"`
	CreatedAt time.Time `json:"createdAt" storm:"index"`
	Result    RunResult `json:"result" storm:"inline"`
	TaskRuns  []TaskRun `json:"taskRuns" storm:"inline"`
}

func (jr *JobRun) ForLogger(kvs ...interface{}) []interface{} {
	output := []interface{}{
		"job", jr.JobID,
		"run", jr.ID,
		"status", jr.Status,
	}

	if jr.Result.HasError() {
		output = append(output, "error", jr.Result.Error())
	}

	return append(kvs, output...)
}

func (jr *JobRun) UnfinishedTaskRuns() []TaskRun {
	unfinished := jr.TaskRuns
	for _, tr := range jr.TaskRuns {
		if tr.Completed() {
			unfinished = unfinished[1:]
		} else if tr.Errored() {
			return []TaskRun{}
		} else {
			return unfinished
		}
	}
	return unfinished
}

func (jr *JobRun) NextTaskRun() TaskRun { return jr.UnfinishedTaskRuns()[0] }

type TaskRun struct {
	Task   Task      `json:"task"`
	ID     string    `json:"id" storm:"id,index,unique"`
	Status string    `json:"status"`
	Result RunResult `json:"result"`
}

func (tr TaskRun) Completed() bool { return tr.Status == StatusCompleted }
func (tr TaskRun) Errored() bool   { return tr.Status == StatusErrored }
func (tr TaskRun) String() string {
	return fmt.Sprintf("TaskRun(%v,%v,%v,%v)", tr.ID, tr.Task.Type, tr.Status, tr.Result)
}

func (tr TaskRun) ForLogger(kvs ...interface{}) []interface{} {
	output := []interface{}{
		"type", tr.Task.Type,
		"params", tr.Task.Params,
		"taskrun", tr.ID,
		"status", tr.Status,
	}

	if tr.Result.HasError() {
		output = append(output, "error", tr.Result.Error())
	}

	return append(kvs, output...)
}

type Output struct {
	Body gjson.Result
}

func (o *Output) Get(path string) gjson.Result {
	return gjson.Get(o.String(), path)
}

func (o *Output) String() string {
	return o.Body.String()
}

func (o *Output) UnmarshalJSON(b []byte) error {
	if !gjson.Valid(string(b)) {
		return fmt.Errorf("invalid JSON: %v", string(b))
	}
	o.Body = gjson.ParseBytes(b)
	return nil
}

func (o *Output) MarshalJSON() ([]byte, error) {
	return []byte(o.Body.String()), nil
}

type RunResult struct {
	Output       *Output     `json:"output"`
	ErrorMessage null.String `json:"error"`
	Pending      bool        `json:"pending"`
}

func RunResultWithValue(val string) RunResult {
	b, err := json.Marshal(map[string]string{"value": val})
	if err != nil {
		logger.Fatal(err)
	}

	var output Output
	if err = json.Unmarshal(b, &output); err != nil {
		logger.Fatal(err)
	}

	return RunResult{Output: &output}
}

func RunResultWithError(err error) RunResult {
	return RunResult{
		ErrorMessage: null.StringFrom(err.Error()),
	}
}

func RunResultPending(input RunResult) RunResult {
	return RunResult{
		Output:       input.Output,
		ErrorMessage: input.ErrorMessage,
		Pending:      true,
	}
}

func (rr RunResult) Get(path string) (gjson.Result, error) {
	if rr.Output == nil {
		return gjson.Result{}, fmt.Errorf("no Output set")
	}
	return rr.Output.Get(path), nil
}

func (rr RunResult) value() (gjson.Result, error) {
	return rr.Get("value")
}

func (rr RunResult) Value() (string, error) {
	val, err := rr.value()
	if err != nil {
		return "", err
	}
	if val.Type != gjson.String {
		return "", fmt.Errorf("non string value")
	}
	return val.String(), nil
}

func (rr RunResult) HasError() bool {
	return rr.ErrorMessage.Valid
}

func (rr RunResult) Error() string {
	return rr.ErrorMessage.String
}

func (rr RunResult) SetError(err error) {
	rr.ErrorMessage = null.StringFrom(err.Error())
}

func (rr RunResult) GetError() error {
	if rr.HasError() {
		return fmt.Errorf("Run Result: ", rr.Error())
	} else {
		return nil
	}
}
