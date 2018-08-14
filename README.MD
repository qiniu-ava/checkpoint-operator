# Checkpoint Operator

Checkpoint operator makes checkpoints (snapshots) for running containers in k8s, and pushes the checkpoints 
to a docker registry for further reuse/restore. It is inspired by [kubepush](https://github.com/wulibin163/kubepush).