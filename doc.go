// Package dockertest is intended to help in executing tests in containers.
//
// This might be simple as executing unit tests in isolation on a docker host.
// You might also want to execute more complex setups like functionally testing a microservice with it's dependencies.
// This is where dockertest supports you.
//
// Basically this package consists of Builders to create a Network or Containers.
// But it also helps in orchestrating the tests and finally in cleaning up.
//
// To get started take a look in the main.go file in the examples folder, it's self explanatory.
//
package dockertest
