# banana-controller
[![Go](https://github.com/middlewaregruppen/banana-controller/actions/workflows/go.yaml/badge.svg)](https://github.com/middlewaregruppen/banana-controller/actions/workflows/go.yaml)

---

Banana is a pattern which allows teams to utilize Kubernetes services through GitOps without all the complexities it would otherwise introduce. It creates an abstraction layer that enables teams to declaratively describe services they want to use, without interfering with their development workflow.

**Work in progress** *banana is still under active development and most features are still in an idéa phase. Please check in from time to time for follow the progress* 🧡

## Description
GitOps is awesome, tools like ArgoCD and Flux are amazing and we love them. But many teams struggle to implement this concept into their workflow for many reasons. A few of them are; They lack the knowledge for the technology required. They want to focus on managing their applications, and not "stuff" around it. They don't feel comfortable making implementation-specific decisions.

Banana focuses on solving these problems by providing the teams a minimal, yet declarative, GitOps-friendly way of listing services they need to exist, which we call `Features`. A feature is typically a Helm chart, but that's all abstracted for the user. Lets imagine that a team wants an ingress controller in their cluster. They create the following:
```yaml
--- 
apiVersion: banana.mdlwr.se/v1alpha1
kind: Feature
metadata:
  name: nginx
spec:
  namespace: nginx
  repo: https://charts.bitnami.com/bitnami
  name: nginx-ingress-controller
  revision: 10.3.0
```
However, the company in this example requires that all HTTP communication must be secured with their internal SSL certificate. Instead of having all teams configuring NGINX with SSL, the "infra-team" can create a `FeatureOverride`. A `FeatureOverride` is a cluster-scoped, policy-like resource that decorates `Features` with additional configuration before it's applied to the cluster. For example:
```yaml
apiVersion: banana.mdlwr.se/v1alpha1
kind: FeatureOverride
metadata:
  name: nginx-default-wildcard-cert
spec:
  featureSelector: {}
  values:
    extraArgs:
      default-ssl-certificate: $(POD_NAMESPACE)/wildcard-cert
```

## Getting Started
Install on your cluster by first applying the CRD's and then the controller
```
kubectl apply -f https://raw.githubusercontent.com/middlewaregruppen/banana-controller/main/deploy/crd.yaml
kubectl apply -f https://raw.githubusercontent.com/middlewaregruppen/banana-controller/main/deploy/deploy.yaml
```

## Contributing
You are welcome to contribute to this project by opening a PR. Create an Issue if you have feedback ❤️
