package command

import (
	"fmt"

	"github.com/kr/pretty"
	//"google.golang.org/grpc/metadata"
	//authzv1 "k8s.io/api/authorization/v1"

	pb "github.com/eparis/remote-shell/api"
	"github.com/eparis/remote-shell/operations/util"
)

// Server is used to implement the RemoteCommandServer
type sndCmd struct{}

// sendCommandAuthz checks if the requestor has permission to run the command
// in question
func (s *sndCmd) sendCommandAuthz(in *pb.CommandRequest, stream pb.RemoteCommand_SendCommandServer) error {
	tokenInfo := util.GetToken(stream.Context())
	pretty.Println(tokenInfo)

	user := tokenInfo.Status.User.Username

	if !tokenInfo.Status.Authenticated || user != "eparis@redhat.com" {
		return fmt.Errorf("user: %v is not authenticated or not eparis", user)
	}
	return nil
}

// SendCommand receives the command from the client and then executes it server-side.
// It returns a commmand reply consisting of the output of the command.
func (s *sndCmd) SendCommand(in *pb.CommandRequest, stream pb.RemoteCommand_SendCommandServer) error {
	var cmdName = in.CmdName
	var cmdArgs = in.CmdArgs

	/*
	   md, ok := metadata.FromIncomingContext(stream.Context())
	   if !ok {
	           return fmt.Errorf("Unable to get metadata from stream")
	   }
	   pretty.Println(md)
	*/
	if err := s.sendCommandAuthz(in, stream); err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(cmdName, cmdArgs, stream)
}

func NewSendCommand() *sndCmd {
	return &sndCmd{}
}
