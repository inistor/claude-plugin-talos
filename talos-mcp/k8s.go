package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/drain"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

const (
	// defaultDrainTimeout matches talosctl's nodedrain.DefaultDrainTimeout.
	defaultDrainTimeout = 5 * time.Minute
	// defaultNodeReadyTimeout is how long to wait for the K8s node to report Ready
	// after the Talos node has come back up.
	defaultNodeReadyTimeout = 5 * time.Minute
	nodeReadyPollInterval   = 5 * time.Second
)

// getKubernetesNodeName resolves the Kubernetes node name from a Talos node by
// reading the k8s.Nodename COSI resource — same approach as talosctl's
// nodedrain.GetKubernetesNodeName. Returns ("", nil) if the resource isn't
// present (node may not be a K8s member yet).
func getKubernetesNodeName(ctx context.Context, c *client.Client) (string, error) {
	res, err := safe.StateGet[*k8s.Nodename](
		ctx,
		c.COSI,
		resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined),
	)
	if err != nil {
		return "", err
	}
	return res.TypedSpec().Nodename, nil
}

// newK8sClientset fetches the cluster's admin kubeconfig via the Talos API
// and builds a typed Kubernetes clientset from it.
func newK8sClientset(ctx context.Context, c *client.Client) (*kubernetes.Clientset, error) {
	kc, err := c.Kubeconfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch kubeconfig: %w", err)
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kc)
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return cs, nil
}

// cordonNode marks the Kubernetes node as unschedulable. Separated from drain
// so the caller can defer the uncordon to run even if drain later fails.
func cordonNode(ctx context.Context, cs *kubernetes.Clientset, nodeName string) error {
	node, err := cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %q: %w", nodeName, err)
	}
	helper := &drain.Helper{Ctx: ctx, Client: cs, Out: io.Discard, ErrOut: io.Discard}
	if err := drain.RunCordonOrUncordon(helper, node, true); err != nil {
		return fmt.Errorf("cordon node %q: %w", nodeName, err)
	}
	return nil
}

// drainNodePods evicts all evictable pods on the named node, matching talosctl's
// `--drain=true` defaults (Force, IgnoreAllDaemonSets, DeleteEmptyDirData,
// pod-grace-period from pod spec). PDB-blocked pods are retried by the drain
// library until the timeout expires. onMsg is called for each eviction so the
// caller can stream updates back to the user.
func drainNodePods(ctx context.Context, cs *kubernetes.Clientset, nodeName string, timeout time.Duration, onMsg func(string)) error {
	if timeout <= 0 {
		timeout = defaultDrainTimeout
	}
	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	helper := &drain.Helper{
		Ctx:                 dCtx,
		Client:              cs,
		Force:               true, // evict unmanaged (bare) pods too
		GracePeriodSeconds:  -1,   // use each pod's own terminationGracePeriodSeconds
		IgnoreAllDaemonSets: true, // DS controller will recreate after reboot anyway
		DeleteEmptyDirData:  true, // node is rebooting; local emptyDir is lost regardless
		Timeout:             timeout,
		Out:                 io.Discard,
		ErrOut:              io.Discard,
		OnPodDeletionOrEvictionStarted: func(pod *corev1.Pod, usingEviction bool) {
			verb := "deleting"
			if usingEviction {
				verb = "evicting"
			}
			onMsg(fmt.Sprintf("%s pod %s/%s", verb, pod.Namespace, pod.Name))
		},
	}
	if err := drain.RunNodeDrain(helper, nodeName); err != nil {
		return fmt.Errorf("drain node %q: %w", nodeName, err)
	}
	return nil
}

// uncordonNode marks the Kubernetes node schedulable again. Best-effort — the
// caller typically wants this to run from a defer so the node doesn't stay
// cordoned if a subsequent step (reboot, wait) fails.
func uncordonNode(ctx context.Context, cs *kubernetes.Clientset, nodeName string) error {
	node, err := cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %q for uncordon: %w", nodeName, err)
	}
	helper := &drain.Helper{Ctx: ctx, Client: cs, Out: io.Discard, ErrOut: io.Discard}
	if err := drain.RunCordonOrUncordon(helper, node, false); err != nil {
		return fmt.Errorf("uncordon node %q: %w", nodeName, err)
	}
	return nil
}

// waitNodeReady polls the Kubernetes node until its Ready condition is True or
// the timeout expires. Transient errors during reboot are tolerated.
func waitNodeReady(ctx context.Context, cs *kubernetes.Clientset, nodeName string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultNodeReadyTimeout
	}
	return wait.PollUntilContextTimeout(ctx, nodeReadyPollInterval, timeout, true, func(pollCtx context.Context) (bool, error) {
		node, err := cs.CoreV1().Nodes().Get(pollCtx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false, nil //nolint:nilerr // transient — node is rebooting
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				return cond.Status == corev1.ConditionTrue, nil
			}
		}
		return false, nil
	})
}

// waitForTalosReboot polls c.Version on the target node and returns once it has
// observed both a failure (node going down) and a recovery (node back up). This
// "saw-down then saw-up" rule guards against the race where a probe lands
// before the reboot has started and would otherwise return success immediately.
func waitForTalosReboot(ctx context.Context, c *client.Client, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	sawDown := false
	pollInterval := 3 * time.Second
	for time.Now().Before(deadline) {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := c.Version(probeCtx)
		cancel()
		if err != nil {
			sawDown = true
		} else if sawDown {
			return nil
		}
		time.Sleep(pollInterval)
	}
	if !sawDown {
		return fmt.Errorf("timeout: node never appeared to go down (reboot may not have been triggered)")
	}
	return fmt.Errorf("timeout: node went down but did not come back up within %s", timeout)
}
