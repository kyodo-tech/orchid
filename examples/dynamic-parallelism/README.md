# dynamic parallelism

In some scenarios we want to run parallel child workflows without knowing upfront how many child workflows we want to run. This is called dynamic parallelism. The example shows this using goroutines and a semaphore to control the number of parallel child workflows.

```
go run .
Starting child workflow '4ea4a9c4-b66a-4c80-acce-3929b00f93a9'
Processing item: item3
Starting child workflow '4f1b1d76-30cb-45ff-a4b1-0b69085d0b80'
Processing item: item1
Starting child workflow '4727ae2c-7af4-4c62-a857-25fb00be70e8'
Processing item: item2
Child workflow '4727ae2c-7af4-4c62-a857-25fb00be70e8' completed
Child workflow '4f1b1d76-30cb-45ff-a4b1-0b69085d0b80' completed
Child workflow '4ea4a9c4-b66a-4c80-acce-3929b00f93a9' completed
Starting child workflow '69bf40d7-eae0-42cd-9a02-5edb78aa6ed0'
Starting child workflow '4a2caf33-5106-4226-97b1-cf2ac1db82ad'
Processing item: item5
Processing item: item4
Child workflow '69bf40d7-eae0-42cd-9a02-5edb78aa6ed0' completed
Child workflow '4a2caf33-5106-4226-97b1-cf2ac1db82ad' completed
Workflow completed. Results:
processing item1
processing item2
processing item3
processing item4
processing item5
```
