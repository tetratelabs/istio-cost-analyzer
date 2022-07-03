# Istio Cost Analyzer

The Istio Cost Analyzer is a tool that allows you to analyze the costliest workload links in your cluster. It relies on Kubernetes/Istio and Prometheus to gather
data, and uses publicly-available cloud egress rates to estimate the overall egress costs of your services.

## Usage

To use this on your kubernetes cluster, make sure you have a kubeconfig in your home directory, and make sure Istio is installed on your cluster, with the prometheus addon enabled.

### Creating `destination_locality` & `source_locality` labels

First, you must create the `destination_locality` & `source_locality` labels for the cost tool to read from.

You can either run the following command and have a webhook handle everything for you:

```
istio-cost-analyzer setupWebhook
```

OR Add the following to all of your deployments:

```yaml
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/extraStatTags: destination_locality
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
                    destination_locality: downstream_peer.labels['locality'].value
            outboundSidecar:
              metrics:
                - name: request_bytes
                  dimensions:
                    destination_locality: upstream_peer.labels['locality'].value
```


### Installation

To install the `istio-cost-analyzer` binary:

```shell
go install github.com/tetratelabs/istio-cost-analyzer@latest
```

You can alternatively clone the repo (`git clone git@github.com:tetratelabs/istio-cost-analyzer.git`) and build the latest
`istio-cost-analyzer` (inside the `istio-cost-analyzer` repo):

```
go install
```

### Running

Run:

```
istio-cost-analyzer analyze
```

This assumes your cluster is on GCP. To change this to the two options of AWS and GCP, run as follows:
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

The output should look like (without `--details`): 

```
Total: <$0.01

SOURCE WORKLOAD	SOURCE LOCALITY	COST   
productpage-v1 	us-west1-b     	<$0.01	
reviews-v2     	us-west1-b     	-     	
reviews-v3     	us-west1-b     	-  
```
With `--details`:

```
Total: <$0.01

SOURCE WORKLOAD	SOURCE LOCALITY	DESTINATION WORKLOAD	DESTINATION LOCALITY	TRANSFERRED (MB)	COST   
productpage-v1 	us-west1-b     	details-v1          	us-west1-c          	0.173250        	<$0.01	
productpage-v1 	us-west1-b     	reviews-v1          	us-west1-b          	0.058500        	-     	
productpage-v1 	us-west1-b     	reviews-v2          	us-west1-b          	0.056250        	-     	
productpage-v1 	us-west1-b     	reviews-v3          	us-west1-b          	0.058500        	-     	
reviews-v2     	us-west1-b     	ratings-v1          	us-west1-b          	0.056150        	-     	
reviews-v3     	us-west1-b     	ratings-v1          	us-west1-b          	0.058400        	-    
```
