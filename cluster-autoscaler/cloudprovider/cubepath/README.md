# Cluster Autoscaler for CubePath

The CubePath cloud provider for the Kubernetes Cluster Autoscaler scales worker nodes in CubePath managed Kubernetes clusters.

## Prerequisites

- A CubePath Kubernetes cluster
- An API token from [my.cubepath.com/account/tokens](https://my.cubepath.com/account/tokens)

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `CUBEPATH_API_TOKEN` | Yes | API bearer token for authentication |
| `CUBEPATH_CLUSTER_UUID` | Yes | UUID of the Kubernetes cluster |
| `CUBEPATH_API_URL` | No | API base URL (default: `https://api.cubepath.com`) |

## How It Works

1. The autoscaler periodically fetches node pools from the CubePath API.
2. When pods are unschedulable, it scales up node pools by calling the scale endpoint.
3. When nodes are underutilized, it removes individual nodes via the delete endpoint.
4. Nodes are matched by provider ID (`cubepath://<vps_id>`) or by node name.

## Deployment

See the [examples](examples/) directory for Kubernetes manifests.

```bash
kubectl create -f examples/cluster-autoscaler-secret.yaml
kubectl create -f examples/cluster-autoscaler-deployment.yaml
```
