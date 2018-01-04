package cat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/eparis/access-daemon/api"
	"github.com/eparis/access-daemon/operations/util"
)

const (
	catName = "cat"
	catExec = "/usr/bin/cat"
)

func init() {
	api.RegisterNewOp(catName, newCatOp)
}

type ClientArgs struct {
	Files []string `json:"files" yaml:"files"`
}

func (ca ClientArgs) valid(permittedFiles []string) error {
	for _, file := range ca.Files {
		found := false
		for _, permittedFile := range permittedFiles {
			if file == permittedFile {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Not permitted to cat file: %s", file)
		}
	}
	return nil
}

func (ca ClientArgs) args() []string {
	return ca.Files
}

type cat struct {
	roleName api.Role
	files    []string
}

func newCatOp(role api.Role, cfgDir string) (api.Operation, error) {
	c := &cat{
		roleName: role,
	}
	var cas []ClientArgs
	if err := util.LoadConfig(cfgDir, &cas); err != nil {
		return nil, err
	}

	for _, ca := range cas {
		c.files = append(c.files, ca.Files...)
	}
	return c, nil
}

func (c *cat) Name() string {
	return catName
}

func (c *cat) Role() api.Role {
	return c.roleName
}

func (c *cat) Go(w http.ResponseWriter, req *http.Request) error {
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

	if err := ca.valid(c.files); err != nil {
		return err
	}

	return util.ExecuteCmdInitNS(w, catExec, ca.args())
}
