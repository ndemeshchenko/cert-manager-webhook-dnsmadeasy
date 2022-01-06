module github.com/ndemeshchenko/cert-manager-webhook-dnsmadeasy

go 1.16

replace github.com/ndemeshchenko/dnsmadeasy => /Users/mykytademeshchenko/projects/dnsmadeasy/

require (
	github.com/jetstack/cert-manager v1.6.1
	github.com/ndemeshchenko/dnsmadeasy v0.0.0-20220105110642-f5ec9c39e519
	k8s.io/api v0.22.3 // indirect
	k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
)

