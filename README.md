# Scylla Operator
> Kubernetes Operator for Scylla (Pre-release version :warning:)

![](https://pbs.twimg.com/media/DwknrKJWkAE7qEQ.jpg)

## Quickstart

To quickly deploy a Scylla cluster on any Kubernetes cluster, follow the [quickstart guide](docs/quickstart.md).


## Description

The scylla-operator is a Kubernetes operator for managing scylla clusters. Currently it supports:
* Deploying multi-zone clusters
* Scaling up or adding new racks
* Scaling down
* Monitoring with Prometheus and Grafana

Future additions include:
* Integration with [Scylla Manager](https://docs.scylladb.com/operating-scylla/manager/)
* Version Upgrade
* Backups
* Restores


## Top-Performance Setup

Scylla performs the best when it has fast disks and direct access to the cpu. To deploy Scylla with maximum performance, follow the guide for your environment:
* [GKE](docs/gke/gke.md)


## Bugs

If you find a bug or need help running scylla, you can reach out in the following ways:
* [Slack](https://scylladb-users-slackin.herokuapp.com/) in the `#scylla-operator` channel.
* File an [issue](https://github.com/kubernetes-sigs/kubebuilder/issues) describing the problem and how to reproduce.


## Acknowledgements

This project is based on cassandra operator, a community effort started by [yanniszark](https://github.com/yanniszark) of [Arrikto](https://www.arrikto.com/), as part of the [Rook project](https://rook.io/).


