module nuodb/nuodb-operator

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/elastic/cloud-on-k8s v0.0.0-20190729075318-8280d4172234
	github.com/elastic/go-elasticsearch/v7 v7.3.0
	github.com/elastic/go-ucfg v0.7.0 // indirect
	github.com/fatih/structs v1.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-ini/ini v1.46.0
	github.com/go-openapi/spec v0.19.0
	github.com/go-test/deep v1.0.3 // indirect
	github.com/integr8ly/grafana-operator v2.0.0+incompatible
	github.com/jonboulle/clockwork v0.1.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/openshift/api v3.9.0+incompatible
	github.com/openshift/client-go v3.9.0+incompatible
	github.com/operator-framework/operator-sdk v0.9.1-0.20190806200632-6c7039c37324
	github.com/sirupsen/logrus v1.4.1
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/pflag v1.0.3
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
	gopkg.in/ini.v1 v1.46.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20190612125737-db0771252981
	k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator v0.0.0-20181213152105-1e8cd453c474
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208
	k8s.io/kubernetes v1.11.8-beta.0.0.20190124204751-3a10094374f2
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.10
)

// Pinned to kubernetes-1.13.4
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
)

replace (
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.29.0
	k8s.io/kube-state-metrics => k8s.io/kube-state-metrics v1.6.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11-0.20190411181648-9d55346c2bde
)

replace github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.9.0

go 1.13
