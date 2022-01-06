# cert-manager-webhook-dnsmadeasy

Cert-manager ACME DNS01 challenge wehook provider for DNS Made Easy.

## Installing

To install with helm, run:

```bash
$ helm repo add dnsmadeasy https://ndemeshchenko.github.io/cert-manager-webhook-dnsmadeeasy
$ helm repo update
$ helm install --name cert-manager-webhook-dnsmadeeasy ndemeshchenko/cert-manager-webhook-dnsmadeeasy
```

or

```bash
$ git clone $thisRepo
$ cd $thisRepoPath
$ helm install --name cert-manager-webhook-dnsmadeeasy .
```

without helm, run:

```bash
$ make rendered-manifest.yaml
$ kubectl apply -f _out/rendered-manifest.yaml
```

### Issuer/ClusterIssuer

An example issuer:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dnsmadeasy-secret
type: Opaque
stringData:
  key: DNSMADEEASY_API_KEY
  secret: DNSMADEEASY_API_SECRET
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-staging
  namespace: default
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: certmaster@dnsmadeasy.com
    privateKeySecretRef:
      name: letsencrypt-staging-account-key
    todo:
```

## Development

### Running the test suite

You can run the test suite with:

1. Go to DNSMadeEasy accotun and get one or create new api token
2. Fill in the appropriate values in `testdata/dnsmadeasy/apikey.yml` and `testdata/dnsmadeasy/config.json` 

```bash
$ TEST_ZONE_NAME=example.com. make test
```
