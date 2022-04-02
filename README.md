# Istio Cost Analyzer

The Istio Cost Analyzer is a tool that allows you to analyze the costliest workload links in your cluster. It relies on Kubernetes/Istio and Prometheus to gather
data, and uses publicly-available cloud egress rates to estimate the overall egress costs of your services.

## Usage

To use this on your kubernetes cluster, make sure you have a kubeconfig in your home directory, and make sure Istio is installed on your cluster, with the prometheus addon enabled.

### Creating `destination_pod`

First, you must create the `destination_pod` metric for the cost tool to read from.

Add the following to all of your deployments:

```yaml
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/extraStatTags: destination_pod
```

Add the following to your Istio Operator:

```yaml
spec:
  values:
    telemetry:
      v2:
        prometheus:
          configOverride:
            inboundSidecar:
              metrics:
                - name: request_bytes
                  dimensions:
                    destination_pod: upstream_peer.name
            outboundSidecar:
              metrics:
                - name: request_bytes
                  dimensions:
                    destination_pod: upstream_peer.name
            gateway:
              metrics:
                - name: request_bytes
                  dimensions:
                    destination_pod: upstream_peer.name
```


### Running

To Build `istio-cost-analyzer` (inside the `istio-cost-analyzer` repo):

```
go install
```

Run:

```
istio-cost-analyzer analyze
```

This assumes your cluster is on GCP. To change this to the two options of AWS and Azure, run as follows:
```
istio-cost-analyzer analyze --cloud aws
```
To point the cost analyzer to your own pricing sheet, run as follows:
```
istio-cost-analyzer analyze --pricePath <path to .json>
```
To only use data from a specific time range, run as follows:
```
istio-cost-analyzer analyze --queryBefore 10h
```
This will only use call data from 10 hours ago and previous.

The output should look like (running with bookinfo with one workload in `eu-west1-b`): 

```
  SOURCE WORKLOAD | SOURCE LOCALITY | DESTINATION WORKLOAD | DESTINATION LOCALITY | TRANSFERRED (MB) |   COST     
------------------+-----------------+----------------------+----------------------+------------------+------------
  productpage-v1  | us-west1-b      | details-v1           | eu-west1-b           |         0.148500 | <$0.01     
  productpage-v1  | us-west1-b      | reviews-v1           | us-west1-b           |         0.049500 | -          
  productpage-v1  | us-west1-b      | reviews-v2           | us-west1-b           |         0.049500 | -          
  productpage-v1  | us-west1-b      | reviews-v3           | us-west1-b           |         0.049500 | -          
  reviews-v2      | us-west1-b      | ratings-v1           | us-west1-b           |         0.049400 | -          
  reviews-v3      | us-west1-b      | ratings-v1           | us-west1-b           |         0.049400 | -          
------------------+-----------------+----------------------+----------------------+------------------+------------
                                                                                         TOTAL       | $0.000012  
                                                                                  -------------------+------------
```
