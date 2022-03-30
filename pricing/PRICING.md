# Pricing Egress
When provided with the "aws" or "gcp" option, Dapani reads from `gcp.json` and `aws.json`
to derive egress rates. If you have a negotiated/different rate, you can modify these
files, and if you have a different cloud/on-prem setup, you may create your own JSON file
that corresponds to this schema and point Dapani to it.

## The Schema

The JSON file it split up into three parts:
 - `inter-zone-intra-region`: Across Zones within a Region
 - `inter-region-intra-continent`: Across Regions within a Continent
 - `inter-continent`: Across Continents

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
charge of $0.01*x.