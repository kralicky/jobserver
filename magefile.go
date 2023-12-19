//go:build mage

package main

import (
	"os"

	"github.com/kralicky/protols/sdk/codegen"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

func Build() error {
	mg.Deps(Generate)

	args := []string{
		"build", "-o", "bin/", "./cmd/...",
	}
	return sh.RunV(mg.GoCmd(), args...)
}

func Generate() error {
	return codegen.GenerateWorkspace()
}

func SampleCerts() error {
	os.RemoveAll("examples/certs")
	if err := os.MkdirAll("examples/certs", 0755); err != nil {
		return err
	}
	commonArgs := []string{
		"-f", "--kty=OKP", "--curve=Ed25519", "--no-password", "--insecure",
	}
	certs := [][]string{
		{"Example CA", "examples/certs/ca.crt", "examples/certs/ca.key", "--profile=root-ca"},
		{"Job Server", "examples/certs/server.crt", "examples/certs/server.key", "--san=localhost", "--san=127.0.0.1", "--profile=leaf", "--ca=examples/certs/ca.crt", "--ca-key=examples/certs/ca.key"},
		{"admin", "examples/certs/admin.crt", "examples/certs/admin.key", "--profile=leaf", "--ca=examples/certs/ca.crt", "--ca-key=examples/certs/ca.key"},
		{"user1", "examples/certs/user.crt", "examples/certs/user.key", "--profile=leaf", "--ca=examples/certs/ca.crt", "--ca-key=examples/certs/ca.key"},
		{"user2", "examples/certs/user.crt", "examples/certs/user.key", "--profile=leaf", "--ca=examples/certs/ca.crt", "--ca-key=examples/certs/ca.key"},
		{"user3", "examples/certs/user.crt", "examples/certs/user.key", "--profile=leaf", "--ca=examples/certs/ca.crt", "--ca-key=examples/certs/ca.key"},
	}

	for _, certArgs := range certs {
		args := []string{"certificate", "create"}
		args = append(args, certArgs...)
		args = append(args, commonArgs...)
		if err := sh.RunV("step", args...); err != nil {
			return err
		}
	}
	return nil
}
