package command

import (
	"fmt"
	"path/filepath"
	"regexp"

	//"github.com/kr/pretty"
	"golang.org/x/net/context"
	//"google.golang.org/grpc/metadata"
	authzv1 "k8s.io/api/authorization/v1"

	rpcapi "github.com/eparis/admin-rpc/api"
	"github.com/eparis/admin-rpc/operations/util"
)

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

type ExecAuth struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Verb      string `json:"verb" yaml:"verb"`
	Resource  string `json:"resource" yaml:"resource"`
	Version   string `json:"version" yaml:"version"`
}
type Exec struct {
	Auth           ExecAuth            `json:"auth" yaml:"auth"`
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

func (exec *Exec) buildRegex() error {
	exec.permittedLong = map[string]argRegex{}
	for flag, vals := range exec.PermittedLong {
		regs, err := stringsToRe(vals)
		if err != nil {
			return err
		}
		exec.permittedLong[flag] = regs
	}
	regs, err := stringsToRe(exec.PermittedNoun)
	if err != nil {
		return err
	}
	exec.permittedNoun = regs
	return nil
}

// authz checks if the requestor has permission to run the command in question
func (exec *Exec) authz(ctx context.Context) error {
	tokenInfo := util.GetToken(ctx)
	clientset := util.GetClientset(ctx)

	// contortions to Change authenticationv1.ExtraValue into authorizationv1.ExtraValue
	// even though they are both just strings :-(
	authnExtras := tokenInfo.Status.User.Extra
	authzExtras := make(map[string]authzv1.ExtraValue, len(authnExtras))
	for key, value := range authnExtras {
		authzExtras[key] = authzv1.ExtraValue(value)
	}
	sar := &authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			User:   tokenInfo.Status.User.Username,
			Groups: tokenInfo.Status.User.Groups,
			UID:    tokenInfo.Status.User.UID,
			Extra:  authzExtras,
			ResourceAttributes: &authzv1.ResourceAttributes{
				Namespace: exec.Auth.Namespace,
				Verb:      exec.Auth.Verb,
				Resource:  exec.Auth.Resource,
				Version:   exec.Auth.Version,
			},
		},
	}

	sar, err := clientset.AuthorizationV1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return err
	}

	if !sar.Status.Allowed {
		return fmt.Errorf("user: %q is not allowed to %q %q in the %q namespace. Refusing", tokenInfo.Status.User.Username, exec.Auth.Verb, exec.Auth.Resource, exec.Auth.Namespace)
	}

	return nil
}

// Server is used to implement the RemoteExecServer
type sndCmd struct {
	commands map[string][]Exec
}

// return the Exec and a bool indicating if it was found
func (s *sndCmd) getExec(cmdName string, cmdArgs []string, ctx context.Context) (Exec, error) {
	commands, ok := s.commands[cmdName]
	if !ok {
		return Exec{}, fmt.Errorf("Command not found: %s", cmdName)
	}
	var firstAuthErr error
	var err error
	for _, cmd := range commands {
		if err = cmd.valid(cmdName, cmdArgs); err == nil {
			if err = cmd.authz(ctx); err == nil {
				// We found a cmd the user could execute. Go Go Go
				return cmd, nil
			}
			// record the auth error
			if firstAuthErr == nil {
				firstAuthErr = err
			}
			continue
		}
	}
	if err == nil {
		err = fmt.Errorf("Somehow we didn't find a command the user could execute and we didn't find an error!\n")
	}
	if firstAuthErr != nil {
		err = firstAuthErr
	}
	return Exec{}, err
}

// SendExec receives the command from the client and then executes it server-side.
// It returns a commmand reply consisting of the output of the command.
func (s *sndCmd) SendExec(in *rpcapi.ExecRequest, stream rpcapi.Exec_SendExecServer) error {
	var cmdName = in.CmdName
	var cmdArgs = in.CmdArgs

	ctx := stream.Context()

	util.AddAuditData(ctx, "command.name", cmdName)
	cmdArgsString := fmt.Sprintf("%#v", cmdArgs)
	util.AddAuditData(ctx, "command.args", cmdArgsString)

	_, err := s.getExec(cmdName, cmdArgs, ctx)
	if err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(cmdName, cmdArgs, stream)
}

func initExecConfig(in interface{}) error {
	exec, ok := in.(*Exec)
	if !ok {
		return fmt.Errorf("initExecConfig called on something other than an Exec!\n")
	}
	if err := exec.buildRegex(); err != nil {
		return err
	}
	return nil
}

func NewExec(cfgDir string) (*sndCmd, error) {
	newCmd := &sndCmd{
		commands: map[string][]Exec{},
	}
	cfgDir = filepath.Join(cfgDir, "command")
	var commandConfigs []Exec
	err := util.LoadConfig(cfgDir, initExecConfig, &commandConfigs)
	if err != nil {
		return nil, err
	}

	if len(commandConfigs) == 0 {
		return nil, fmt.Errorf("No commands defined in: %s\n", cfgDir)
	}

	for _, cmd := range commandConfigs {
		cmdName := cmd.CmdName
		newCmd.commands[cmdName] = append(newCmd.commands[cmdName], cmd)
	}
	return newCmd, nil
}
