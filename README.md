# SNI proxy fault injection implementation

Istio fault injection only applies to HTTP routes. I wanted to do fault injection on HTTPS. To accomplish this, I'll use a proxy to do the fault injection, and I'll use istio to route the traffic through the proxy

## Run via helm

edit the `values.yaml` file to specify the namespace of the workload, the urls on which you'd like to inject faults, and the fault injection rate. If a faultDelay is set higher than 0, then we'll delay the request. If the faultDelay is 0, we'll fail it immediately

With this repo as your current directory, use helm to install the chart- `helm install -n <helm-release-namespace> fault-injection chart -f chart/values.yaml`

The virtualservice will route all requests to the specified URLs through our SNI proxy. The SNI proxy will inject failures at the specified rate.

The proxy workload is configured to bypass istio for external egress

## test locally via curl

run the proxy as a container
```
docker run -e FAULT_INJECTION_RATE=0.1 -e FAULT_INJECTION_SLEEP=2 -p 443:443 jonwoodlief/faultinjection:latest
```

target the container via curl
```
curl --resolve <hostname>:443:127.0.0.1 https://<hostname>/<route>
```


## source

most of the SNI proxy implementation code taken from here: https://www.agwa.name/blog/post/writing_an_sni_proxy_in_go
modifications were made to inject intermittent faults
