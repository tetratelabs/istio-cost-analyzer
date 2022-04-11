# Pricing Egress

The cost tool reads rates in a "flat" format, which means:

```json
{
  "us-west1-b": {
    "us-west1-c": 0.05
  }
}
```
Here, the first entry (`us-west1-b`) is the call origin, and the nested entry (`us-west1-c`) is the call
destination. The value to that is the egress rate in $/GB.

The cost tool pulls the flat files `aws/aws_pricing.json` and `gcp/gcp_pricing.json` from GitHub at runtime.

## Custom Pricing

You can use `format_converter.go` to transform a somewhat generalized and structured egress pricing 
structure to the flat one (flat structures can go on for thousands of lines). To do this, put
your egress pricing in the following schema:
 - `inter-zone-intra-region`: Across Zones within a Region
 - `inter-region-intra-continent`: Across Regions within a Continent
 - `inter-continent`: Across Continents

The GCP rates exist in this format (`gcp/gcp.json`), and are converted to a flat pricing scheme that the cost 
analyzer can read in `gcp/gcp_rate_converter.go` (See below). The format above can also be generalized to any
custom rates, as long as `gcp_rate_converter.go` is provided a file with prices in that structure.

Each of these fields hold a sub-object, which holds the rates for all
of the regions. **Rates are in American dollars per Gigabyte.** 
An example from 
`gcp.json` is:
```json
"inter-zone-intra-region": {
    "us": "0.01",
    "northamerica": "0.01",
    "eu": "0.01",
    "asia": "0.01",
    "southamerica": "0.01",
    "australia": "0.01"
  }
```
This means, for example, if `us-west-1` calls `us-west-2` for `x` GB, there is an egress
charge of $0.01*x. You would repeat this for `inter-region-intra-continent` and `inter-continent`.

After this, you can run `format_converter.go` like so (replace `--in` and `--out` with your values):

```shell
go run pricing/gcp/gcp_rate_converter.go --in pricing/gcp/gcp.json --out pricing/gcp/gcp_pricing.json
```

Where `pricing/gcp.json` holds structured rates and `pricing/gcp_pricing.json` holds outputted flat rates. 