# ormon - Openshift Route Monitor

Automated Openshift Route monitoring.

## Usage

Configuration is handled via a single yaml file and kubeconfigs:

```yaml
targets:
  - kubeconfig: /etc/openshift-route-exporter/devcluster.kubeconfig
    labels:
      cluster: devcluster
  - kubeconfig: /etc/openshift-route-exporter/prodcluster.kubeconfig
    labels:
      cluster: prodcluster
```

One can use the small "script" in helper to create a kubeconfig.

## Annotations

* `thobits.com/ormon-skip`: Set this to any of `1`, `t`, `T`, `TRUE`, `true` or `True` to skip monitoring a Route.
* `thobits.com/ormon-method`: Set http method the check the Route.
* `thobits.com/ormon-valid-statuscodes`: Configure valid statuscodes, multiple can be comma seperated.
* `thobits.com/ormon-body-regex`: Body validation regex
