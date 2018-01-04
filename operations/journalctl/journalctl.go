package journalctl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/eparis/access-daemon/api"
	"github.com/eparis/access-daemon/operations/util"
)

const (
	journalName = "journalctl"
	journalExec = "/usr/bin/journalctl"
)

func init() {
	api.RegisterNewOp(journalName, newJournalOp)
}

type ClientArgs struct {
	Since  string   `json:"since,omitempty"`
	Until  string   `json:"until,omitempty"`
	Units  []string `json:"units,omitempty"`
	Follow bool     `json:"follow,omitempty"`
}

func (j ClientArgs) valid() error {
	return nil
}

func (j ClientArgs) args() []string {
	args := []string{}
	if j.Since != "" {
		args = append(args, fmt.Sprintf("--since=%s", j.Since))
	}

	if j.Until != "" {
		args = append(args, fmt.Sprintf("--until=%s", j.Until))
	}

	for _, unit := range j.Units {
		args = append(args, fmt.Sprintf("--unit=%s", unit))
	}

	if j.Follow {
		args = append(args, "-f")
	}
	return args
}

type journal struct {
	role api.Role
}

func newJournalOp(role api.Role, cfgDir string) (api.Operation, error) {
	j := &journal{
		role: role,
	}
	return j, nil
}

func (j *journal) Name() string {
	return journalName
}

func (j *journal) Role() api.Role {
	return j.role
}

func (j journal) Go(w http.ResponseWriter, req *http.Request) error {
	var jArgs ClientArgs

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		err = json.Unmarshal(data, &jArgs)
		if err != nil {
			return err
		}
	}

	if err := jArgs.valid(); err != nil {
		return err
	}

	args := jArgs.args()

	return util.ExecuteCmdInitNS(w, journalExec, args)
}
