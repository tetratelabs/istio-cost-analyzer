# Istio Cost Analyzer

The Istio Cost Analyzer is a tool that allows you to analyze the costliest workload links in your cluster. It relies on Kubernetes/Istio and Prometheus to gather
data, and uses publicly-available cloud egress rates to estimate the overall egress costs of your services.

## Usage

To use this on your kubernetes cluster, make sure you have a kubeconfig in your home directory, and make sure Istio is installed on your cluster, with the prometheus addon enabled. You must also have a `HEALTHY` Istio Operator available.


### Installation

To install the `istio-cost-analyzer` binary:

```shell
go install github.com/tetratelabs/istio-cost-analyzer@latest
```

### Setup

The setup command does a few things:
- Edits Istio Operator config to add custom prometheus metrics (a `destination_locality` label on an Istio metric).
- Creates a Mutating Webhook that gets called when a new deployment is created. This mutating webhook runs in a pod and has associated RBAC permissions, Services, etc.
- Labels existing pods & deployments in said `--targetNamespace`.

You can either run the following command and have a webhook handle everything all existing Deployments and all Deployments created in the future:

```
istio-cost-analyzer setup --targetNamespace <ns>
```

| Flag                |                                             Description                                             |           Default Value |
|:--------------------|:---------------------------------------------------------------------------------------------------:|------------------------:|
| targetNamespace     |                        Namespace which the cost analyzer will watch/analyze                         |               `default` |
| cloud               | Cloud on which your cluster is running (node info varies cloud to cloud -- inferred from Node info) | Inferred from Node info |
| analyzerNamespace   |       Namespace in which cost analyzer config will exist (you usually don't need to set this)       |          `istio-system` |


## Running

Run:

```
istio-cost-analyzer analyze
```

| Flag                |                                                                           Description                                                                           |           Default Value |
|:--------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------:|------------------------:|
| cloud               |                              Cloud on which your cluster is running (node info varies cloud to cloud). Options are `gcp` or `aws`.                              | Inferred from Node info |
| prometheusNamespace |                                        Namespace in which the prometheus pod exists (you usually don't need to set this)                                        |          `istio-system` |
| pricePath           | For non-standard aws/gcp rates (on-prem, negotiated rates). If you set this, you don't need to set `cloud`. See `/pricing` (you usually don't need to set this) |                    None |
| details             |                              Extended table view that shows both destination and source workload/locality, instead of just source.                              |                 `false` |


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

### Cleanup

If you want to restart installation of the tool or don't want it in your cluster anymore, you can run:
    
```
istio-cost-analyzer destroy
```

You must set the `--analyzerNamespace` flag if you set it in the `setup` command.

You must also edit your Istio Operator config to remove the custom prometheus metrics. (you can use `-o` to do that here, but it's unstable)