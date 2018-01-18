package command

import (
	"fmt"
	"path/filepath"
	"regexp"

	//"github.com/kr/pretty"
	"golang.org/x/net/context"
	//"google.golang.org/grpc/metadata"
	authzv1 "k8s.io/api/authorization/v1"

	pb "github.com/eparis/remote-shell/api"
	"github.com/eparis/remote-shell/operations/util"
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

type CommandAuth struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Verb      string `json:"verb" yaml:"verb"`
	Resource  string `json:"resource" yaml:"resource"`
	Version   string `json:"version" yaml:"version"`
}
type Command struct {
	Auth           CommandAuth         `json:"auth" yaml:"auth"`
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

func (cmd *Command) buildRegex() error {
	cmd.permittedLong = map[string]argRegex{}
	for flag, vals := range cmd.PermittedLong {
		regs, err := stringsToRe(vals)
		if err != nil {
			return err
		}
		cmd.permittedLong[flag] = regs
	}
	regs, err := stringsToRe(cmd.PermittedNoun)
	if err != nil {
		return err
	}
	cmd.permittedNoun = regs
	return nil
}

// authz checks if the requestor has permission to run the command in question
func (cmd *Command) authz(ctx context.Context) error {
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
				Namespace: cmd.Auth.Namespace,
				Verb:      cmd.Auth.Verb,
				Resource:  cmd.Auth.Resource,
				Version:   cmd.Auth.Version,
			},
		},
	}

	sar, err := clientset.AuthorizationV1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return err
	}

	if !sar.Status.Allowed {
		return fmt.Errorf("user: %q is not allowed to %q %q in the %q namespace. Refusing", tokenInfo.Status.User.Username, cmd.Auth.Verb, cmd.Auth.Resource, cmd.Auth.Namespace)
	}

	return nil
}

// Server is used to implement the RemoteCommandServer
type sndCmd struct {
	commands map[string][]Command
}

// return the Command and a bool indicating if it was found
func (s *sndCmd) getCommand(cmdName string, cmdArgs []string, ctx context.Context) (Command, error) {
	commands, ok := s.commands[cmdName]
	if !ok {
		return Command{}, fmt.Errorf("Command not found: %s", cmdName)
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
	return Command{}, err
}

// SendCommand receives the command from the client and then executes it server-side.
// It returns a commmand reply consisting of the output of the command.
func (s *sndCmd) SendCommand(in *pb.CommandRequest, stream pb.RemoteCommand_SendCommandServer) error {
	var cmdName = in.CmdName
	var cmdArgs = in.CmdArgs

	ctx := stream.Context()

	util.AddAuditData(ctx, "command.name", cmdName)
	cmdArgsString := fmt.Sprintf("%#v", cmdArgs)
	util.AddAuditData(ctx, "command.args", cmdArgsString)

	/*
	   md, ok := metadata.FromIncomingContext(stream.Context())
	   if !ok {
	           return fmt.Errorf("Unable to get metadata from stream")
	   }
	   pretty.Println(md)
	*/
	_, err := s.getCommand(cmdName, cmdArgs, ctx)
	if err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(cmdName, cmdArgs, stream)
}

func initCommandConfig(in interface{}) error {
	cmd, ok := in.(*Command)
	if !ok {
		return fmt.Errorf("initCommandConfig called on something other than a Command!\n")
	}
	if err := cmd.buildRegex(); err != nil {
		return err
	}
	return nil
}

func NewSendCommand(cfgDir string) (*sndCmd, error) {
	newCmd := &sndCmd{
		commands: map[string][]Command{},
	}
	cfgDir = filepath.Join(cfgDir, "command")
	var commandConfigs []Command
	err := util.LoadConfig(cfgDir, initCommandConfig, &commandConfigs)
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
