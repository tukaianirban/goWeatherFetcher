# goWeatherFetcher

This is a simple application written mostly in Go to demonstrate some powerful features , ways of programming and advantages of using Golang over other languages. The application serves out the current weather in a given city.
There are 3 modules here:
1. A Web API which accepts web GET requests (not really a REST API) with 'location' argument containing the name of the city.
2. A RabbitMQ based messaging bus, which is deployed as a container using the rabbitmq:latest image.
3. A fetcher service that receives user requests from the messaging bus, parses out the 'location', fetches current weather of the location from the OpenWeather Api, and then returns the response (weather forecast) back to the messaging bus.

The web api and fetcher service are standalone native Golang applications, and communicate with each other using the RabbitMQ messaging bus, which is deployed as a containerized service.

The web service shows how to use channels for asynchronous and pooled communication between goroutines. The http hander routines (that handle the web request) send a struct to a 'dispenser' routine containing a channel.
The fetcher service uses a pool of routines to fetch weather data from the OpenWeather API. It also shows how asynchronous communication between a pool of routines and another pool / single routine can be done using a dispenser,responder routines.

The code here demonstrates the different techniques that can be used when designing and inter-connecting services / micro-services written in Golang. RabbitMQ and Kafka are popular messaging buses, and there is an increased trend to containerize them. Hence, the motivation to keep RMQ in a docker container here. The techniques here can be expanded/extended to adapt to scaling systems with services being geographically and platform-wise separated.

To make queries to the web api, I used cURL. A sample call when done from the same host machine is:
$ curl -v --noproxy "*" "http://localhost:8881/?location=Berlin,DE"
