# Caching workflow step example

This example demonstrates using a `orchid.Snapshotter` to cache data in a workflow for efficient retries.

```
go run .
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=fetchData
Fetching data from external source...
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=fetchData
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=checkpoint
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=checkpoint
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=processData
Processing failed, will retry with cached data.
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed with route" activity=processData route=checkpoint
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=checkpoint
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=checkpoint
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=processData
Processing data successfully: initial_data
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=processData
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=snapshotDelete
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=snapshotDelete
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Starting activity" activity=complete
Workflow completed successfully.
time=2024-10-31T00:30:10.098+09:00 level=INFO msg="Activity completed" activity=complete
Final Workflow Output - Data: processed_data, Rating: 60
```
