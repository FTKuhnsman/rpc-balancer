# RPC Balancer

RPC Balancer is a Go application that acts as a proxy for Ethereum RPC endpoints. It accepts a variable number of RPC endpoints as command-line flags and starts a web server listening on a specified port. When an RPC request is sent to the service, it attempts to proxy that request to the pool of RPC endpoints. If a request fails, the corresponding RPC endpoint is marked as unhealthy, and the service moves on to the next one. If all endpoints are unhealthy, the request is sent to a fallback endpoint.

## Features

- Accepts a variable number of RPC endpoints as command-line flags.
- Proxies RPC requests to a pool of endpoints.
- Implements automatic failover logic.
- Allows specifying a fallback endpoint. If not set, the fallback will default to the first endpoint provided
- Provides a web server for handling RPC requests.
- Supports a separate metrics server for monitoring.
- **Health check feature:** By setting the health check interval, the service periodically polls unhealthy endpoints with an `eth_blockHeight` request to check if they start responding again. If successful, the endpoint is set as healthy and added back to the pool.

## Installation From Source

To install and run RPC Balancer, follow these steps:

1. Clone the repository:

```
git clone https://github.com/your-username/rpc-balancer.git
```

2. Navigate to the project directory:

```
cd rpc-balancer
```

3. Build the application:

```
go build
```


## Running the application

```
./rpc-balancer -port <rpc_port> -metricsport <metrics_port> -healthcheckinterval <interval> -node <rpc_endpoint_1> -node <rpc_endpoint_2> ... -fallback <fallback_endpoint>
```
Replace `<rpc_port>`, `<metrics_port>`, `<interval>`, `<rpc_endpoint_1>`, `<rpc_endpoint_2>`, and `<fallback_endpoint>` with your desired values.

Within your blockchain application set the RPC url to the IP address of the machine running the service and defined port: (hostname:port)

## Usage

RPC Balancer accepts the following command-line flags:

- `-rpcport`: Port to run the server on (default 8080).
- `-metricsport`: Port to run the metrics server on (default 8081).
- `-fallback`: Fallback node to use (default: none).
- `-healthcheckinterval`: Interval in seconds to check node health (default 5).
- `-node`: Node to add to the pool. This flag can be repeated to add multiple nodes.

Example usage:

```
./rpc-balancer -port 8080 -metricsport 8081 -healthcheckinterval 10 -node http://rpc1.example.com -node http://rpc2.example.com -fallback http://fallback.example.com
```

## Docker Installation

RPC Balancer is also available as a Docker container, making it easy to deploy in containerized environments. You can either pull the pre-built image from Docker Hub or build the image locally using the provided Dockerfile.

### Pulling the Image from Docker Hub

You can pull the latest Docker image from Docker Hub using the following command:

```
docker pull ftkuhnsman/rpc-balancer:latest
```

### Building the Image Locally

If you prefer to build the Docker image locally, you can use the provided Dockerfile. Here are the steps to build the image:

1. Clone the repository:

```
git clone https://github.com/your-username/rpc-balancer.git
```

2. Navigate to the project directory:

```
cd rpc-balancer
```

3. Build the Docker image:

```
docker build -t rpc-balancer .
```

### Running with Docker Compose

A Docker Compose file is included in the repository to simplify the deployment process. Here's how you can use Docker Compose to run RPC Balancer:

1. Create a `docker-compose.yml` file with the following content:

```
yaml
version: '3'

services:
  rpcbalancer:
    image: ftkuhnsman/rpc-balancer:latest
    ports:
      - 8080:8080
      - 8081:8081
    command: |
      -port 8080
      -metricsport 8081
      -node https://arb1.arbitrum.io/rpc
      -node https://rpc.ankr.com/arbitrum
      -healthcheckinterval 10
```

2. Run Docker Compose:

```
docker-compose up
```

## Metrics

RPC Balancer exposes several metrics to monitor the health and performance of the service:

- **num_requests:** Total number of requests processed by method.
- **num_failover_requests:** Total number of failover requests by endpoint.
- **num_requests_invalid:** Total number of invalid requests.

These metrics can be accessed via a Prometheus metrics server. By default, the metrics server runs on port 8081. You can view the metrics by navigating to `http://localhost:8081` in your web browser or by querying the Prometheus server directly.

## Docker

The application works on Docker

## Contributing

Contributions are welcome! If you find a bug or have an idea for an improvement, please open an issue or submit a pull request.

## License

This project is licensed under the BSD3 License - see the [LICENSE](LICENSE) file for details.