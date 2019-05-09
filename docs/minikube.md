# Deploying Scylla on Minikube

The easiest and quickest way to try Scylla on Kubernetes!

## Prerequisites

* [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)

## Walkthrough


### Start Minikube:

```bash
minikube start
```

By default, Minikube uses 2 CPUs and 2Gi RAM.


### Deploy the Scylla Operator:

```bash
kubectl apply -f examples/minikube/operator.yaml
```

This will install the operator StatefulSet in namespace scylla-operator-system. You can check if the operator is up and running with:
 
```bash
kubectl -n scylla-operator-system get pods
```
 

### Create a Scylla Cluster

```bash
kubectl create -f examples/minikube/cluster.yaml
```

We can verify that a Kubernetes object has been created that represents our new Scylla cluster with the command below.
This is important because it shows that  has successfully extended Kubernetes to make Scylla clusters a first class citizen in the Kubernetes cloud-native environment.

```bash
kubectl -n scylla get clusters.scylla.scylladb.com
```

You can also track the state of a Scylla cluster from its status. To check the current status of a Cluster, run:

```bash
kubectl -n scylla describe clusters.scylla.scylladb.com simple-cluster
```

### Accessing the Database

* From kubectl:

To get a cqlsh shell in your new Cluster:
```console
kubectl exec -n scylla -it simple-cluster-east-1-east-1a-0 -- cqlsh
> DESCRIBE KEYSPACES;
```

### Notice

This guide deploys Scylla is aimed at simplicity and because of that,
Scylla is deployed with sub-optimal performance settings.

For deploying Scylla with the optimal configuration, see the more advanced
[GKE guide](gke.md).