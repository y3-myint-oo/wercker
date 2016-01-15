package main

import (
	"testing"

	"github.com/wercker/sentcli/util"
)

func TestBuildEnvironment(t *testing.T) {
	env := util.NewEnvironment("X_FOO=bar", "BAZ=fizz")
	passthru := env.GetPassthru().Ordered()
	if len(passthru) != 1 {
		t.Fatal("Expected only one variable in passthru")
	}
	if passthru[0][0] != "FOO" {
		t.Fatal("Expected to find key 'FOO'")
	}
	if passthru[0][1] != "bar" {
		t.Fatal("Expected to find value 'bar'")
	}
}
