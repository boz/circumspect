package kube

import "k8s.io/api/core/v1"

type Props interface {
	KubeNamespace() string
	KubePodName() string
	KubeLabels() map[string]string
	KubeAnnotations() map[string]string
	KubeContainerName() string
}

type props struct {
	pod *v1.Pod
	cs  *v1.ContainerStatus
}

func newProps(pod *v1.Pod, cs *v1.ContainerStatus) Props {
	return props{pod, cs}
}

func (p props) KubeNamespace() string {
	return p.pod.Namespace
}

func (p props) KubePodName() string {
	return p.pod.Name
}

func (p props) KubeLabels() map[string]string {
	return p.pod.Labels
}

func (p props) KubeAnnotations() map[string]string {
	return p.pod.Labels
}

func (p props) KubeContainerName() string {
	return p.cs.Name
}
