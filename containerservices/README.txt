To start the RabbitMQ messaging bus as a container:
$ docker run -dit --name rabbitmq rabbitmq

Once the container is started:
$ docker ps -a --filter "name=rabbitmq*"
CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS                                NAMES
2af237c854c3        rabbitmq            "docker-entrypoint.s…"   2 days ago          Up 2 days           4369/tcp, 5671-5672/tcp, 25672/tcp   rabbitmq

Check the ports that the service has exposed:
$ docker inspect rabbitmq --format '{{ .Config.ExposedPorts }}'
map[25672/tcp:{} 4369/tcp:{} 5671/tcp:{} 5672/tcp:{}]

Check the IP address on which the service can be reached:
[root@r620-015 web]# docker inspect rabbitmq --format '{{ .NetworkSettings.Networks.bridge.IPAddress }}'
172.17.0.2


Some RabbitMQ control commands, and their use when in containerized services:
$ docker exec -it rabbitmq rabbitmqctl status
$ docker exec -it rabbitmq rabbitmqctl list_exchanges
$ docker exec -it rabbitmq rabbitmqctl list_channels
$ docker exec -it rabbitmq rabbitmqctl list_channels messages_uncommitted
$ docker exec -it rabbitmq rabbitmqctl list_connections
