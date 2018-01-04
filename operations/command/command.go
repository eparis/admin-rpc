package command

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/eparis/access-daemon/api"
	"github.com/eparis/access-daemon/operations/util"
)

const (
	commandOpName = "command"
)

func init() {
	api.RegisterNewOp(commandOpName, newCommandOp)
}

type ClientArgs struct {
	CmdName string   `json:"CmdName"`
	Args    []string `json:"args,omitempty"`
}

func (ca ClientArgs) args() []string {
	return ca.Args
}

func (ca ClientArgs) valid(cmd Command) error {
	err := parseArgs(ca.Args, cmd)
	if err != nil {
		return err
	}

	return nil
}

type argRegex []*regexp.Regexp

func (a argRegex) valid(val string) bool {
	if len(a) == 0 && val == "" {
		return true
	}
	for _, v := range a {
		if v.MatchString(val) {
			return true
		}
	}
	return false
}

type Command struct {
	CmdName        string              `json:"cmdName" yaml:"cmdName"`
	Required       []string            `json:"requiredFlags,omitempty" yaml:"requiredFlags,omitempty"`
	PermittedShort []string            `json:"permittedShortFlags,omitempty" yaml:"permittedShortFlags,omitempty"`
	PermittedLong  map[string][]string `json:"permittedLongFlags,omitempty" yaml:"permittedLongFlags,omitempty"`
	permittedLong  map[string]argRegex
	PermittedNoun  []string `json:"permittedNouns,omitempty" yaml:"permittedNouns,omitempty"`
	permittedNoun  argRegex
}

func stringsToRe(in []string) (argRegex, error) {
	regs := make(argRegex, 0, len(in))
	for _, val := range in {
		re, err := regexp.Compile(val)
		if err != nil {
			return nil, err
		}
		regs = append(regs, re)
	}
	return regs, nil
}

func (cmdConfig *Command) buildRegex() error {
	cmdConfig.permittedLong = map[string]argRegex{}
	for flag, vals := range cmdConfig.PermittedLong {
		regs, err := stringsToRe(vals)
		if err != nil {
			return err
		}
		cmdConfig.permittedLong[flag] = regs
	}
	regs, err := stringsToRe(cmdConfig.PermittedNoun)
	if err != nil {
		return err
	}
	cmdConfig.permittedNoun = regs
	return nil
}

type commandOp struct {
	role     api.Role
	commands []Command
}

func newCommandOp(role api.Role, cfgDir string) (api.Operation, error) {
	c := &commandOp{
		role: role,
	}

	var configs []Command
	err := util.LoadConfig(cfgDir, &configs)
	if err != nil {
		return nil, err
	}

	if len(configs) == 0 {
		fmt.Printf("  %s is useless because no config found in %s\n", commandOpName, cfgDir)
	}

	for _, config := range configs {
		cmdName := config.CmdName
		if _, ok := c.getCommand(cmdName); ok {
			return nil, fmt.Errorf("Same command registered twice: %s\n", cmdName)
		}
		err := config.buildRegex()
		if err != nil {
			return nil, err
		}
		c.commands = append(c.commands, config)
	}
	return c, nil
}

func (c *commandOp) Name() string {
	return commandOpName
}

func (c *commandOp) Role() api.Role {
	return c.role
}

func (c *commandOp) getCommand(cmdName string) (Command, bool) {
	for _, cmd := range c.commands {
		if cmd.CmdName == cmdName {
			return cmd, true
		}
	}
	return Command{}, false
}

func (c *commandOp) Go(w http.ResponseWriter, req *http.Request) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}

	var ca ClientArgs
	if len(data) > 0 {
		err = json.Unmarshal(data, &ca)
		if err != nil {
			return err
		}
	}

	op, found := c.getCommand(ca.CmdName)
	if !found {
		return fmt.Errorf("Command not found: %s", ca.CmdName)
	}

	if err := ca.valid(op); err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(w, ca.CmdName, ca.args())
}
