# Snapshot Operator

Snapshot operator makes snapshots for containers running in k8s, and pushes the snapshots 
to a docker registry for further reuse/restore. It is inspired by [kubepush](https://github.com/wulibin163/kubepush).

## Snapshot

```yaml
apiVersion: "ava.qiniu.com/v1alpha1"
kind: "Snapshot"
metadata:
  name: "example-snapshot"
spec:
  podName: "my-pod"
  containerName: "my-container"
  # full name of the image to the desired snapshot, 
  # including registry host, tag and anything needed to push/pull an image
  imageName: "my-private-registry.com/my-snapshot:v0.0.1"
  # secret used to push the snapshot-ed image, same as an imagePullSecret
  imagePushSecret:
    name: "my-docker-registry-secret"
```

## How to use

0. You will need a running kubernetes cluster with access to, Minikube or Docker for Mac for example.

1. Create custom resource definition for snapshot:
    ```bash
    $ kubectl create -f deploy/crd.yaml
    ```

2. Start snapshot operator
    ```bash
    $ kubectl create -f deploy/operator.yaml
    ```

3. Create a snapshot
    ```bash
    $ kubectl create -f deploy/cr.yaml
    ```

4. You can now use kubectl to check the newly created snapshot. A job in the name of `example-snapshot-xxxxx` could be observed after a while, it is created by the snapshot operator to perform the docker committing and docker pushing work. 
    ```bash
    $ kubectl get snapshot
    NAME                 AGE
    example-snapshot   22s

    $ kubectl get job
    NAME                       DESIRED   SUCCESSFUL   AGE
    example-snapshot-bjpf4   1         0            37s
    ```