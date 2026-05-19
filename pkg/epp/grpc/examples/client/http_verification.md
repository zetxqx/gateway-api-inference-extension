# Useful Commands

## Notes

* Github branch eppgrpcexp
* Current EPP is using static IP address passed by a flag. We may need to 

### Setup
```sh
# mygrpc is for grpc -> grpc
# mygrpc2 is for http -> grpc
docker build -t us-central1-docker.pkg.dev/bobzetian-gke-dev/gateway-api-inference-extension/epp:mygrpc2 .
docker push us-central1-docker.pkg.dev/bobzetian-gke-dev/gateway-api-inference-extension/epp:mygrpc2

# Using the following namespace for setup.
export NS=llmd-standalone
k delete deployment gaie-inference-scheduling-epp -n $NS
k apply -f pkg/epp/grpc/examples/standalone.yaml -n $NS
```

### Configure

### Testing

For grpc -> grpc
```sh
go run ./pkg/epp/grpc/examples/client --target-address "34.169.180.33:8081"
```

For http -> grpc
```sh
IP=34.82.61.76
curl -i http://${IP}:8081/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{
  "model": "'"${MODEL_NAME}"'",
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ],
  "max_completion_tokens": 10
}'

curl -i http://${IP}:8081/v1/models

curl -i -N http://${IP}:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"${MODEL_NAME}"'",
    "messages": [
      {
        "role": "user",
        "content": "Hello, world!"
      }
    ],
    "stream": true,
    "stream_options": {
      "include_usage": true
    }
  }'
```

For http -> http (via EPP)
```sh
IP=34.187.145.211
curl -i http://${IP}:8081/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{
  "model": "'"${MODEL_NAME}"'",
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ],
  "max_completion_tokens": 10
}'

curl -i http://${IP}:8081/v1/models

curl -i -N http://${IP}:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"${MODEL_NAME}"'",
    "messages": [
      {
        "role": "user",
        "content": "Hello, world!"
      }
    ],
    "stream": true,
    "stream_options": {
      "include_usage": true
    }
  }'
```

for pure http -> http
```sh
IP=136.117.50.55
curl -i http://${IP}:8081/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{
  "model": "'"${MODEL_NAME}"'",
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ],
  "max_completion_tokens": 10
}'
```
