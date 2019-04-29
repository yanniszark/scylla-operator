package identity

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	log "github.com/sirupsen/logrus"
	"github.com/yanniszark/go-nodetool/nodetool"
	corev1 "k8s.io/api/core/v1"
	"math/rand"
	"net"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Member encapsulates the identity for a single member
// of a Scylla Cluster.
type Member struct {
	// Name of the Pod
	Name string
	// Namespace of the Pod
	Namespace string
	// IP of the Pod
	IP string
	// ClusterIP of the member's Service
	StaticIP     string
	Rack         string
	Datacenter   string
	Cluster      string
	Bootstrapped bool
}

func Retrieve(name, namespace string, client client.Client) (*Member, error) {

	// Get the member's service
	var memberService *corev1.Service
	const maxRetryCount = 5

	for retryCount := 0; ; retryCount++ {
		memberService = &corev1.Service{}
		err := client.Get(context.TODO(), naming.NamespacedName(name, namespace), memberService)
		if err == nil {
			break
		}
		if retryCount > maxRetryCount {
			return nil, errors.Wrap(err, "failed to get memberservice")
		}
		log.Errorf("Something went wrong trying to get Member Service %s: %+v", name, err)
		time.Sleep(time.Second)
	}

	// Get the pod's ip
	pod := &corev1.Pod{}
	err := client.Get(context.TODO(), naming.NamespacedName(name, namespace), pod)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod")
	}

	m := &Member{
		Name:       name,
		Namespace:  namespace,
		IP:         pod.Status.PodIP,
		StaticIP:   memberService.Spec.ClusterIP,
		Rack:       pod.Labels[naming.RackNameLabel],
		Datacenter: pod.Labels[naming.DatacenterNameLabel],
		Cluster:    pod.Labels[naming.ClusterNameLabel],
	}

	bootstrapped, err := m.IsBootstrapped(client)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	m.Bootstrapped = bootstrapped

	return m, nil
}

func (m *Member) GetSeeds(kubeClient client.Client) ([]string, error) {

	var services []corev1.Service
	var err error

	const maxRetry = 5
	for i := 0; i < maxRetry; i++ {

		services, err = func() ([]corev1.Service, error) {
			services := &corev1.ServiceList{}
			err := kubeClient.List(context.TODO(), &client.ListOptions{LabelSelector: naming.SeedsSelector(m.Cluster)}, services)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if len(services.Items) == 0 {
				return nil, errors.New("No seeds were found.")
			}

			return services.Items, nil
		}()

		if err != nil {
			seeds := []string{}
			for _, svc := range services {
				seeds = append(seeds, svc.Spec.ClusterIP)
			}
			return seeds, nil
		}
	}

	return nil, err
}

func (m *Member) IsBootstrapped(client client.Client) (bool, error) {

	// Get corresponding cluster
	c := &scyllav1alpha1.Cluster{}
	err := client.Get(context.TODO(), naming.NamespacedName(m.Name, m.Namespace), c)
	if err != nil {
		return false, errors.WithStack(err)
	}

	// Resolve client service to get IPs
	ips, err := net.LookupIP(
		fmt.Sprintf(
			"%s.%s",
			naming.HeadlessServiceNameForCluster(c),
			m.Namespace,
		),
	)
	if len(ips) == 0 {
		return false, nil
	}
	if err != nil {
		return false, errors.WithStack(err)
	}

	rand.Seed(time.Now().Unix())
	var bootstrapped bool
	const maxRetry = 5

	for i := 0; i < maxRetry; i++ {

		bootstrapped, err = func() (bool, error) {
			// Sleep a little to let the state propagate
			time.Sleep(time.Second)

			// Choose a random Member of the ring
			ip := ips[rand.Intn(len(ips))].String()
			var addr *url.URL
			addr, err = url.Parse(naming.JolokiaAddressForHost(ip))

			// Get the ring information
			var nodeMap nodetool.NodeMap
			nodeMap, err = nodetool.NewFromURL(addr).Status()
			if err != nil {
				return false, errors.WithStack(err)
			}

			// Check if our ip is part of the ring.
			// The Member's UUID must also be non-empty.
			for _, node := range nodeMap {
				if node.Host == m.IP && len(node.ID) != 0 {
					return true, nil
				}
			}
			return false, nil
		}()

		// If Member was found, return immediately.
		if bootstrapped {
			return true, nil
		}
	}

	return bootstrapped, errors.WithStack(err)
}
