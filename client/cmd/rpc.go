package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"

	pb "github.com/eparis/admin-rpc/api"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	namespace = "eparis"
)

var (
	_ = pretty.Print
)

// getPods returns all running pods in a map from nodename to pod
func getPods(clientset *kubernetes.Clientset, namespace string) (map[string]corev1.Pod, error) {
	podList, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := podList.Items
	if len(pods) < 1 {
		return nil, fmt.Errorf("No pods found in namespace: %s\n", namespace)
	}
	out := make(map[string]corev1.Pod)

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		out[pod.Spec.NodeName] = pod
	}
	return out, nil
}

func getNodes(pods map[string]corev1.Pod) []string {
	nodes := make([]string, 0, len(pods))
	for nodeName := range pods {
		nodes = append(nodes, nodeName)
	}
	sort.Strings(nodes)
	return nodes
}

func ForwardToPod(kubeConfig *rest.Config, pod corev1.Pod) error {
	contentConfig := dynamic.ContentConfig()
	dc, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return err
	}
	serverGroups, err := dc.ServerGroups()
	if err != nil {
		return err
	}
	var preferredVersion *metav1.GroupVersionForDiscovery
	for _, group := range serverGroups.Groups {
		if group.Name == "" {
			preferredVersion = &group.PreferredVersion
			break
		}
	}
	if preferredVersion == nil {
		return fmt.Errorf("Unable to find PreferredVersion for group \"\"")
	}
	contentConfig.GroupVersion = &schema.GroupVersion{
		Group:   "api",
		Version: preferredVersion.Version,
	}
	kubeConfig.ContentConfig = contentConfig

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if stopChan != nil {
			close(stopChan)
		}
	}()

	restClient, err := rest.RESTClientFor(kubeConfig)
	if err != nil {
		return err
	}
	req := restClient.Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod.ObjectMeta.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(kubeConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	ports := []string{fmt.Sprintf("%d", port)}
	fw, err := portforward.New(dialer, ports, stopChan, readyChan, nil, os.Stderr)
	if err != nil {
		return err
	}
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			log.Fatalf("Unable to ForwardPorts: %v", err)
		}
	}()
	<-readyChan
	return nil
}

func attachToken(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
	return metautils.NiceMD(md).ToOutgoing(ctx)
}

func getClientset() (*rest.Config, *kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to load kubeconfig: %v\n", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	return kubeConfig, clientset, nil
}

func GetGRPCClient(node string) (pb.RemoteCommandClient, context.Context, error) {
	kubeConfig, clientset, err := getClientset()
	if err != nil {
		return nil, nil, err
	}
	token := kubeConfig.BearerToken

	pods, err := getPods(clientset, namespace)
	if err != nil {
		return nil, nil, err
	}

	pod, ok := pods[node]
	if !ok {
		return nil, nil, fmt.Errorf("Unable to find pod on node: %s", node)
	}

	fmt.Printf("Connecting to node: %s\n", node)

	if err = ForwardToPod(kubeConfig, pod); err != nil {
		return nil, nil, fmt.Errorf("Unable to forward to target pod: %v\n", err)
	}

	creds, err := credentials.NewClientTLSFromFile("certs/CA.crt", "rpc.eparis.svc")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create TLS credentials %v", err)
	}
	dopts := []grpc.DialOption{grpc.WithDefaultCallOptions()}
	dopts = append(dopts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.Dial(serverAddr, dopts...)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not connect: %v", err)
	}

	// Create the client
	c := pb.NewRemoteCommandClient(conn)

	ctx := context.Background()
	ctx = attachToken(ctx, token)

	return c, ctx, nil
}
