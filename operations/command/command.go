package command

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/kr/pretty"
	"golang.org/x/net/context"
	//"google.golang.org/grpc/metadata"
	authzv1 "k8s.io/api/authorization/v1"

	pb "github.com/eparis/remote-shell/api"
	"github.com/eparis/remote-shell/operations/util"
)

var (
	allCommands = []Command{}
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

// Load loads the commands from the configuration directory specified
func Load(cfgDir string) error {
	cfgDir = filepath.Join(cfgDir, "command")
	var commandConfigs []Command
	err := util.LoadConfig(cfgDir, &commandConfigs)
	if err != nil {
		return err
	}

	if len(commandConfigs) == 0 {
		return fmt.Errorf("No commands defined in: %s\n", cfgDir)
	}

	for _, cmd := range commandConfigs {
		cmdName := cmd.CmdName
		fmt.Printf("Processing Regexp For: %s\n", cmdName)
		if _, ok := getCommand(cmdName, []string{}); ok {
			return fmt.Errorf("Same command registered twice: %s\n", cmdName)
		}
		err := cmd.buildRegex()
		if err != nil {
			return err
		}
		allCommands = append(allCommands, cmd)
	}
	return nil
}

// return the Command and a bool indicating if it was found
func getCommand(cmdName string, cmdArgs []string) (Command, bool) {
	for _, cmd := range allCommands {
		if cmd.CmdName == cmdName {
			return cmd, true
		}
	}
	return Command{}, false
}

// Server is used to implement the RemoteCommandServer
type sndCmd struct{}

// authz checks if the requestor has permission to run the command in question
func (s *sndCmd) authz(in *pb.CommandRequest, ctx context.Context) error {
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
				Namespace: "default",
				Verb:      "get",
				Resource:  "pods",
				Version:   "v1",
			},
		},
	}

	sar, err := clientset.AuthorizationV1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return err
	}

	if !sar.Status.Allowed {
		return fmt.Errorf("user: %v is not allowed to get pods in the default namespace. Refusing", tokenInfo.Status.User.Username)
	}

	return nil
}

// SendCommand receives the command from the client and then executes it server-side.
// It returns a commmand reply consisting of the output of the command.
func (s *sndCmd) SendCommand(in *pb.CommandRequest, stream pb.RemoteCommand_SendCommandServer) error {
	fmt.Printf("HERE!\n")
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
	cmd, found := getCommand(cmdName, cmdArgs)
	if !found {
		return fmt.Errorf("Command not found: %s", cmdName)
	}

	pretty.Println(cmd)
	if err := cmd.valid(cmdName, cmdArgs); err != nil {
		return err
	}

	if err := s.authz(in, ctx); err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(cmdName, cmdArgs, stream)
}

func NewSendCommand() *sndCmd {
	return &sndCmd{}
}
