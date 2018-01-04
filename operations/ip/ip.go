package ip

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/eparis/access-daemon/api"
	"github.com/eparis/access-daemon/operations/util"
)

const (
	ipName = "ip"
	ipExec = "/usr/sbin/ip"
)

func init() {
	api.RegisterNewOp(ipName, newIPOp)
}

type ClientArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func (ca ClientArgs) valid() error {
	cmd := ca.Command
	if cmd != "addr" && cmd != "neigh" {
		return fmt.Errorf("Command: %s not permitted, only ip 'addr' and 'neigh'", cmd)
	}
	args := ca.Args
	if len(args) > 1 {
		return fmt.Errorf("Too many arguments, only 'show' permitted")
	}
	return nil
}

func (ca ClientArgs) args() []string {
	args := []string{}
	args = append(args, ca.Command)
	args = append(args, ca.Args...)
	return args
}

type ipOp struct {
	role api.Role
}

func newIPOp(role api.Role, cfgDir string) (api.Operation, error) {
	ip := &ipOp{
		role: role,
	}
	return ip, nil
}

func (ip *ipOp) Name() string {
	return ipName
}

func (ip *ipOp) Role() api.Role {
	return ip.role
}

func (ip *ipOp) Go(w http.ResponseWriter, req *http.Request) error {
	var ca ClientArgs

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		err = json.Unmarshal(data, &ca)
		if err != nil {
			return err
		}
	}

	err = ca.valid()
	if err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(w, ipExec, ca.args())
}
