package command

import (
	"fmt"

	"github.com/kr/pretty"
	"golang.org/x/net/context"
	//"google.golang.org/grpc/metadata"
	authzv1 "k8s.io/api/authorization/v1"

	pb "github.com/eparis/remote-shell/api"
	"github.com/eparis/remote-shell/operations/util"
)

// Server is used to implement the RemoteCommandServer
type sndCmd struct{}

// sendCommandAuthz checks if the requestor has permission to run the command
// in question
func (s *sndCmd) sendCommandAuthz(in *pb.CommandRequest, ctx context.Context) error {
	tokenInfo := util.GetToken(ctx)
	clientset := util.GetClientset(ctx)
	pretty.Println(tokenInfo)

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
	if err := s.sendCommandAuthz(in, ctx); err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(cmdName, cmdArgs, stream)
}

func NewSendCommand() *sndCmd {
	return &sndCmd{}
}
