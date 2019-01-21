# Deploying Scylla on GKE

This guide is focused on deploying Scylla on GKE with maximum performance. It sets up the kubelets on GKE nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd) in RAID0 for maximum performance.

Because this guide focuses on showing a glimpse of the true performance of Scylla, we use 32 core machines with local SSDs. This might be overkill if all you want is a quick setup to play around with scylla operator. If you just want to quickly set up a Scylla cluster for the first time, we suggest you look at the [quickstart guide](quickstart.md) first.

## TL;DR;

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# From inside the docs/gke folder 
./gke.sh [GCP user] [GCP project] [GCP zone]

# Example:
# ./gke.sh yanniszark@arrikto.com gke-demo-226716 us-west1-b
```

After you deploy, see how you can [benchmark your cluster with cassandra-stress]().

## Walkthrough


### Google Kubernetes Engine Setup

#### Creating a GKE cluster

For this guide, we'll create a GKE cluster with the following:

1. A NodePool of 3 `n1-standard-32` Nodes, where the Scylla Pods will be deployed. Each of these Nodes has 8 local SSDs attached, which will later be combined into a RAID0 array. It is important to disable `autoupgrade` and `autorepair`, since they cause loss of data on local SSDs. 

```
gcloud beta container --project "${GCP_PROJECT}" \
clusters create "${CLUSTER_NAME}" --username "admin" \
--zone "${GCP_ZONE}" \
--cluster-version "1.11.6-gke.2" \
--node-version "1.11.6-gke.2" \
--machine-type "n1-standard-32" \
--num-nodes "5" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-labels role=scylla-clusters \
--image-type "UBUNTU" \
--enable-cloud-logging --enable-cloud-monitoring \
--no-enable-autoupgrade --no-enable-autorepair
```

2. A NodePool of 2 `n1-standard-32` Nodes to deploy `cassandra-stress` later on.

```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "1.11.6-gke.2" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-labels role=cassandra-stress \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

3. A NodePool of 1 `n1-standard-8` Node, where the operator and the monitoring stack will be deployed.
```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "operator-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "1.11.6-gke.2" \
--machine-type "n1-standard-8" \
--num-nodes "1" \
--disk-type "pd-ssd" --disk-size "20" \
--node-labels role=scylla-operator \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

#### Setting Yourself as `cluster-admin`
> (By default GKE doesn't give you the necessary RBAC permissions)

Get the credentials for your new cluster
```
gcloud container clusters get-credentials "${CLUSTER_NAME}" --zone="${GCP_ZONE}"
```

Create a ClusterRoleBinding for you
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"
```


### Installing Required Tools 

#### Installing Helm

Helm is needed to enable multiple features. If you don't have Helm installed in your cluster, follow this:

1. Go to the [helm docs](https://docs.helm.sh/using_helm/#installing-helm) to get the binary for your distro.
2. `helm init`
3. Give Helm `cluster-admin` role:
```
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
```

#### Install RAID DaemonSet

To combine the local disks together in RAID0 arrays, we deploy a DaemonSet to do the work for us.

```
kubectl apply -f examples/gke/raid-daemonset.yaml
```

#### Install the local provisioner

After combining the local SSDs into RAID0 arrays, we deploy the local volume provisioner, which will discover their mount points and make them available as PersistentVolumes.
```
helm install --name local-provisioner examples/gke/provisioner
```

#### Install `cpu-policy` Daemonset

Scylla achieves top-notch performance by pinning the CPUs it uses. To enable this behaviour in Kubernetes, the kubelet must be configured with the [static CPU policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/). To configure the kubelets in the `scylla` and `cassandra-stress` NodePools, we deploy a DaemonSet to do the work for us. You'll notice the Nodes getting cordoned for a little while, but then everything will come back to normal.
```
kubectl apply -f examples/gke/cpu-policy-daemonset.yaml
```

### Installing the Scylla Operator

```
kubectl apply -f examples/gke/operator.yaml
```

Spinning up Scylla Cluster!

```
kubectl apply -f examples/gke/simple_cluster.yaml
```

Check the status of your cluster

```
kubectl describe cluster simple-cluster -n scylla
```

### Setting up Monitoring

Both Prometheus and Grafana were configured to work out-of-the-box with Scylla Operator. Both of them will be available under the `monitoring` namespace. If you want to customize them, you can edit `prometheus/values.yaml` and `grafana/values.yaml` then run the following commands:

1. Install Prometheus
```
helm upgrade --install scylla-prom --namespace monitoring examples/gke/prometheus
```

2. Install Grafana
```
helm upgrade --install scylla-graf --namespace monitoring examples/gke/grafana
```

To see Grafana locally, run:

```
export POD_NAME=$(kubectl get pods --namespace monitoring -l "app=grafana,release=scylla-graf" -o jsonpath="{.items[0].metadata.name}")
kubectl --namespace monitoring port-forward $POD_NAME 3000
```

And access `http://0.0.0.0:3000` from your browser.

:warning: Keep in mind that Grafana needs Prometheus DNS to be visible to get information. The Grafana available in this files was configured to work with the name `scylla-prom` and `monitoring` namespace. You can edit this configuration under `grafana/values.yaml`.


## Benchmark with cassandra-stress

After deploying our cluster along with the monitoring, we can benchmark it using cassandra-stress and see its performance in Grafana. We have a mini cli that generates Kubernetes Jobs that run cassandra-stress against a cluster.

> Because cassandra-stress doesn't scale well to multiple cores, we use multiple jobs with a small core count for each

```bash

# Run a benchmark with 10 jobs, with 6 cpus and 50.000.000 operations each.
# Each Job will throttle throughput to 30.000 ops/sec for a total of 300.000 ops/sec.
scripts/cass-stress-gen.py --num-jobs=10 --cpu=6 --memory=20G --ops=50000000 --limit=30000 --nodeselector role=cassandra-stress
kubectl apply -f scripts/cassandra-stress.yaml
```

While the benchmark is running, open up Grafana and take a look at the monitoring metrics.

After the Jobs finish, clean them up with:
```bash
kubectl delete -f ./cassandra-stress.yaml
```