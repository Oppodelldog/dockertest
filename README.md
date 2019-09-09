# dockertest
[![GoDoc](https://godoc.org/github.com/Oppodelldog/dockertest?status.svg)](https://godoc.org/github.com/Oppodelldog/dockertest)
[![Go Report Card](https://goreportcard.com/badge/github.com/Oppodelldog/dockertest)](https://goreportcard.com/report/github.com/Oppodelldog/dockertest)

This project is an experimental library that wraps docker client to support testing services in docker containers.

I split it out from my docker-dns project where I found the need for functional testing a dns-container behaving well after build.

The rough concept is as follows:
* create a ```ContainerBuilder``` (which takes basic parameters)
* if necessay modify the docker-client data structures which are exposed by the builder
* use the builder to create a ```Container```
* use ```Container``` to start

Additional to the creation and starting of containers there are convenicence methods like
```WaitForContainerToExit``` which waits for the container executing tests,

For debugging those tests it is useful to use method ```DumpContainerLogs``` to take a look inside the components under test.

Finally ```Cleanup()``` the whole setup, jenkins will love you for that. 
