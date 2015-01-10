package main

import (
	"testing"
)

func TestBuildEnvironment(t *testing.T) {
	env := NewEnvironment([]string{"X_FOO=bar", "BAZ=fizz"})
	passthru := env.getPassthru()
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
