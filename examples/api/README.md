# API - functional tests

Let's assume you wrote some micro service exposing a simple REST API.  
Let's further assume this API will be a 'name store' - So you can **PUT** names into it and **GET** a list of all names.  

in directory **nameapi** the microservice is implemented.  
In directory **tests** the test is implemented.
Directory **healthcheck** contains some simple healthcheck that will help docker to determine that **nameapi** is available.
The test-orchestration code will wait for **nameapi** to be healthy before starting the tests.   

Take a look at **[main.go](main.go)** which implements the test orchestration using **dockertest**.

