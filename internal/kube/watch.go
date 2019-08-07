package kube

import (
	"context"
	"net/url"
	"regexp"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	csroutev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	// Labels stores kubernetes labels
	Labels map[string]string

	// Annotations stores ose related annotations
	Annotations map[string]string

	// Config to create a watcher
	Config struct {
		Kubeconfig          string `yaml:"kubeconfig"`
		NamespaceBlackRegex string `yaml:"namespace_blacklist_regex"`
		Labels              Labels `yaml:"labels"`
	}

	// Watcher watcher monitors a cluster for route events
	Watcher struct {
		kubeconfig          string
		config              *rest.Config
		clientset           *csroutev1.RouteV1Client
		cache               cache.Store
		controller          cache.Controller
		Labels              Labels
		NamespaceBlackRegex *regexp.Regexp
	}

	// ResourceEventHandlerFuncs is an adaptor to let you easily specify as many or
	// as few of the notification functions as you want while still implementing
	// ResourceEventHandler.
	ResourceEventHandlerFuncs struct {
		AddFunc    func(obj *Route)
		UpdateFunc func(oldObj, newObj *Route)
		DeleteFunc func(obj *Route)
	}
)

// NewWatcher crates a Wachter
func NewWatcher(c Config) (w *Watcher, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
	if err != nil {
		return
	}
	clientset, err := csroutev1.NewForConfig(config)
	if err != nil {
		return
	}
	if c.NamespaceBlackRegex == "" {
		c.NamespaceBlackRegex = "^$"
	}

	cache, controller := cache.NewInformer(
		cache.NewListWatchFromClient(
			clientset.RESTClient(), "routes", corev1.NamespaceAll, fields.Everything(),
		),
		&routev1.Route{},
		10*time.Minute,
		cache.ResourceEventHandlerFuncs{},
	)

	re, err := regexp.Compile(c.NamespaceBlackRegex)
	if err != nil {
		return
	}

	return &Watcher{
		kubeconfig:          c.Kubeconfig,
		config:              config,
		clientset:           clientset,
		cache:               cache,
		controller:          controller,
		Labels:              c.Labels,
		NamespaceBlackRegex: re,
	}, nil
}

// Watch nonblocking all events from openshift and throw them into c
func (w *Watcher) Watch(ctx context.Context) {
	// consume or wait for context cancel
restarter:
	for {
		w.controller.Run(ctx.Done())
		select {
		// restart after 10 seconds
		case <-time.After(10 * time.Second):
			continue restarter
		case <-ctx.Done():
			break restarter
		}
	}
}

// List availibe Routes
func (w *Watcher) List() (routes []*Route) {
	routeInterfaces := w.cache.List()
	routes = []*Route{}
	host, _ := url.Parse(w.config.Host)
	for _, ri := range routeInterfaces {
		rv1 := ri.(*routev1.Route)
		r := Route{rv1, host.Host}
		if w.validRoute(&r) {
			routes = append(routes, &r)
		}
	}
	return
}

func (w *Watcher) validRoute(r *Route) bool {
	valid := !w.NamespaceBlackRegex.MatchString(r.ObjectMeta.Namespace)
	return valid
}
