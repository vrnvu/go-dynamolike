# go-dynamolike

A DynamoDB-like database implementation in Go.

## Project Structure

- server: HTTP server implementation 
- partition: Consistent hashing partition implementation using Jump Consistent Hash algorithm
- discovery: Service discovery implementation using Docker to discover running containers in our target network
- client: Client implementation for interacting with the DynamoDB-like database, through MinIO's S3 API
- storage: MinIO as our backend storage solution

## Getting Started

### Prerequisites

- Go 1.22.5 or later
- Docker (for local development)

### Running the Project

```
make
curl -X PUT -d "hello world" localhost:3000/object/id-1
curl -X GET localhost:3000/object/id-1
```
