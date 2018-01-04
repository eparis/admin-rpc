package util

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

// LoadConfig takes a directory and a pointer to a slice of types. For every
// file in the directory which ends in .yaml or .yml it will decode that file
// into and append the results to the slice provided
func LoadConfig(cfgDir string, ref interface{}) error {
	files, err := ioutil.ReadDir(cfgDir)
	if err != nil {
		return err
	}

	// Allocate slice with desired capacity
	slice := reflect.ValueOf(ref).Elem()
	slice.Set(reflect.MakeSlice(slice.Type(), 0, 0))

	for _, file := range files {
		path := filepath.Join(cfgDir, file.Name())
		if !file.Mode().IsRegular() {
			fmt.Printf("  Skipping non-file: %s\n", path)
		}
		// Every directory has a "config" file, ignore that
		if filepath.Base(path) == "config" {
			continue
		}
		// probably should check for *.yaml or *.yml
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			fmt.Printf("  Not loading config (invalid suffix): %s\n", path)
			continue
		}
		fmt.Printf("  Loading file: %s\n", path)
		cfg, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		data := reflect.New(slice.Type().Elem())
		if err := yaml.Unmarshal(cfg, data.Interface()); err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, data.Elem()))

	}
	return nil
}
