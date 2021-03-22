module gardener/subnet

go 1.13

require (
	gardener/networkGlobal v0.0.0-00010101000000-000000000000
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
)

replace gardener/networkGlobal => github.com/onmetal/network-basics/networkGlobal v0.0.0-20210317165632-fd7bef291c78
