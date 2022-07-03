# Istio Cost Analyzer

The Istio Cost Analyzer is a tool that allows you to analyze the costliest workload links in your cluster. It relies on Kubernetes/Istio and Prometheus to gather
data, and uses publicly-available cloud egress rates to estimate the overall egress costs of your services.

## Usage

To use this on your kubernetes cluster, make sure you have a kubeconfig in your home directory, and make sure Istio is installed on your cluster, with the prometheus addon enabled.


### Installation

To install the `istio-cost-analyzer` binary:

```shell
go install github.com/tetratelabs/istio-cost-analyzer@latest
```

### Creating `destination_locality` label

You must create the `destination_locality` label for the cost tool to read from.

You can either run the following command and have a webhook handle everything all existing Deployments and all Deployments created in the future:

```
istio-cost-analyzer setup
```

The setup command will also add a locality label to every pod in your chosen namespaces, which is necessary for the tool.

OR Add the following to all of your Kubernetes Deployments now and in the future:

```yaml
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/extraStatTags: destination_locality
```

### Operator Setup

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



## Running

Run:

```
istio-cost-analyzer analyze
```

This assumes your cluster is on GCP. To change this to the two options of AWS and GCP, run as follows:
```
istio-cost-analyzer analyze --cloud aws
```
To point the cost analyzer to your own pricing sheet, run as follows (takes local files and urls):
```
istio-cost-analyzer analyze --pricePath <path to .json>
```

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
