# Webhook Example

This example demonstrates how to use asynchronous execution above a first acknowledge step with randomized failures and retries.

```
go run examples/webhook/webhook.go
Loaded workflow from file
time=2024-03-31T12:06:54.115+09:00 level=ERROR msg="failed to log workflow step" activity=B attempt=0 delay=1.000000001s error="random failure"
time=2024-03-31T12:06:55.116+09:00 level=ERROR msg="failed to log workflow step" activity=B attempt=1 delay=3s error="random failure"
time=2024-03-31T12:06:58.118+09:00 level=ERROR msg="failed to log workflow step" activity=B attempt=2 delay=1m0s error="random failure"
A
time=2024-03-31T12:07:58.120+09:00 level=ERROR msg="failed to log workflow step" activity=C attempt=0 delay=1.000000001s error="random failure"
time=2024-03-31T12:07:59.121+09:00 level=ERROR msg="failed to log workflow step" activity=C attempt=1 delay=3s error="random failure"
AB
ABC
ABCDE
```

Is printed if we hit the server with:

```
curl localhost:8080/a -X POST -d 'A'
AB
```

Idempotency key example via header:

```
curl localhost:8080/webhook -X POST -d 'D' -H 'request-id: test'
execution ID test: execution ID already exists
```
