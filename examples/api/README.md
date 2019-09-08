# API - functional tests

Let's assume you wrote some micro service exposing a simple REST API.  
This API will be a name store, so you can **PUT** names into it and **GET** a list of all names.  

in directory **nameapi** the microservice is implemented.  
In directory **tests** the test is implemented.   

**main.go** implements the functional test using **dockertest**

