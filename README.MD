# Checkpoint Operator

Checkpoint operator makes checkpoints (snapshots) for containers running in k8s, and pushes the checkpoints 
to a docker registry for further reuse/restore. It is inspired by [kubepush](https://github.com/wulibin163/kubepush).

## Checkpoint

```yaml
apiVersion: "ava.qiniu.com/v1alpha1"
kind: "Checkpoint"
metadata:
  name: "example-checkpoint"
spec:
  podName: "my-pod"
  containerName: "my-container"
  # full name of the image to the desired checkpoint, 
  # including registry host, tag and anything needed to push/pull an image
  imageName: "my-private-registry.com/my-checkpoint:v0.0.1"
  # secret used to push the checkpoint-ed image, same as an imagePullSecret
  imagePushSecret:
    name: "my-docker-registry-secret"
```

## How to use

0. You will need a running kubernetes cluster with access to, Minikube or Docker for Mac for example.

1. Create custom resource definition for checkpoint:
    ```bash
    $ kubectl create -f deploy/crd.yaml
    ```

2. Start checkpoint operator
    ```bash
    $ kubectl create -f deploy/operator.yaml
    ```

3. Create a checkpoint
    ```bash
    $ kubectl create -f deploy/cr.yaml
    ```

4. You can now use kubectl to check the newly created checkpoint. A job in the name of `example-checkpoint-xxxxx` could be observed after a while, it is created by the checkpoint operator to perform the docker committing and docker pushing work. 
    ```bash
    $ kubectl get checkpoint
    NAME                 AGE
    example-checkpoint   22s

    $ kubectl get job
    NAME                       DESIRED   SUCCESSFUL   AGE
    example-checkpoint-bjpf4   1         0            37s
    ```