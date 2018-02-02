package command

import (
	"fmt"
	"strings"
)

type parseOp struct {
	exec       *Exec
	foundFlags []string
}

func (po *parseOp) checkShort(flag string) error {
	po.foundFlags = append(po.foundFlags, flag)
	for _, short := range po.exec.PermittedShort {
		if flag == short {
			return nil
		}
	}
	return fmt.Errorf("Shorthand flag not permitted: -%s", flag)
}

func (po *parseOp) checkLongVal(flag, value string) error {
	po.foundFlags = append(po.foundFlags, flag)
	regs, ok := po.exec.permittedLong[flag]
	if !ok {
		return fmt.Errorf("Long flag not permitted: --%s", flag)
	}
	if len(regs) == 0 && value == "" {
		return nil
	}
	if regs.valid(value) {
		return nil
	}
	return fmt.Errorf("Flag value not permitted: --%s=%s", flag, value)
}

func (po *parseOp) checkNoun(noun string) error {
	if po.exec.permittedNoun.valid(noun) {
		return nil
	}
	return fmt.Errorf("Noun not permitted: %s", noun)
}

func (po *parseOp) checkRequiredFlags() error {
	for _, reqFlag := range po.exec.Required {
		found := false
		for _, foundFlag := range po.foundFlags {
			if foundFlag == reqFlag {
				found = true
				break
			}
		}
		if !found {
			if len(reqFlag) == 1 {
				return fmt.Errorf("Required flag not found: -%s", reqFlag)
			}
			return fmt.Errorf("Required flag not found: --%s", reqFlag)
		}
	}
	return nil
}

func (po *parseOp) parseLongArg(s string) error {
	name := s[2:]
	// This is a special case where someone used `--` as an argument. Usually
	// this indicates to stop processing the remaining things as arguments
	// but we don't allow pipelines like that. So I'm just going to deny
	// all cases of `--`. This maybe should change someday?
	if len(name) == 0 {
		return fmt.Errorf("Argument is invalid: %s", "--")
	}

	// Check for `---` or `--=` both are bad...
	if name[0] == '-' || name[0] == '=' {
		return fmt.Errorf("bad arg syntax: %s", s)
	}

	split := strings.SplitN(name, "=", 2)

	flag := split[0]

	var value string
	if len(split) == 2 {
		// '--flag=arg'
		value = split[1]
	}

	return po.checkLongVal(flag, value)
}

// "shorthands" can be a series of shorthand letters of flags (e.g. "-vvv").
// short arguments can not take values. It is an error is there is an "="
// short arguments will always be parsed as since flags. So all of these may
// fail....
// -f=FILENAME  # Fail because of =
// -fFILENAME   # parsed as series of flags: f, F, I, L, E, ...
// -f FILENAME  # parsed as 1 short flags "f" and 1 noun: "FILENAME"
func (po *parseOp) parseShortArgs(s string) error {
	shorthands := s[1:]
	if len(shorthands) == 0 {
		return fmt.Errorf("bad arg syntax: %s", s)
	}
	if strings.Contains(shorthands, "=") {
		return fmt.Errorf("Setting values not permitted with short flags: %s", s)
	}

	for len(shorthands) > 0 {
		flag := shorthands[:1]
		shorthands = shorthands[1:]

		if err := po.checkShort(flag); err != nil {
			return err
		}
	}
	return nil
}

func (exec *Exec) valid(cmdName string, cmdArgs []string) error {
	po := parseOp{
		exec:       exec,
		foundFlags: []string{},
	}
	var err error
	for len(cmdArgs) > 0 {
		s := cmdArgs[0]
		cmdArgs = cmdArgs[1:]
		if len(s) == 0 || s[0] != '-' || len(s) == 1 {
			if err := po.checkNoun(s); err != nil {
				return err
			}
			continue
		}

		if s[1] == '-' {
			err = po.parseLongArg(s)
		} else {
			err = po.parseShortArgs(s)
		}
		if err != nil {
			return err
		}
	}
	return po.checkRequiredFlags()
}
