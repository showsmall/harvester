package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kv1 "kubevirt.io/client-go/api/v1"

	"github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/pkg/util"
)

const (
	repoVMNamePrefix = "upgrade-repo-"
	repoVMUserData   = `name: "enable repo mode"
stages:
  rootfs:
  - commands:
    - echo > /sysroot/harvester-serve-iso
`
	repoServiceNamePrefix = "upgrade-repo-"
)

type HarvesterRelease struct {
	Harvester       string `yaml:"harvester,omitempty"`
	HarvesterChart  string `yaml:"harvesterChart,omitempty"`
	OS              string `yaml:"os,omitempty"`
	Kubernetes      string `yaml:"kubernetes,omitempty"`
	Rancher         string `yaml:"rancher,omitempty"`
	MonitoringChart string `yaml:"monitoringChart,omitempty"`
}

type UpgradeRepoInfo struct {
	Release HarvesterRelease
}

func (info *UpgradeRepoInfo) Marshall() (string, error) {
	out, err := yaml.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (info *UpgradeRepoInfo) Load(data string) error {
	return yaml.Unmarshal([]byte(data), info)
}

type UpgradeRepo struct {
	ctx     context.Context
	upgrade *harvesterv1.Upgrade
	h       *upgradeHandler

	httpClient *http.Client
}

func NewUpgradeRepo(ctx context.Context, upgrade *harvesterv1.Upgrade, upgradeHandler *upgradeHandler) *UpgradeRepo {
	return &UpgradeRepo{
		ctx:     ctx,
		upgrade: upgrade,
		h:       upgradeHandler,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (r *UpgradeRepo) Bootstrap() error {
	image := r.upgrade.Status.ImageID
	if image == "" {
		return errors.New("Upgrade repo image is not provided")
	}
	upgradeImage, err := r.GetImage(image)
	if err != nil {
		return err
	}

	_, err = r.createVM(upgradeImage)
	if err != nil {
		return err
	}

	_, err = r.createService()
	return err
}

func getISODisplayNameImageName(upgradeName string, version string) string {
	return fmt.Sprintf("%s-%s", upgradeName, version)
}

func (r *UpgradeRepo) CreateImageFromISO(isoURL string, checksum string) (*harvesterv1.VirtualMachineImage, error) {
	imageSpec := &harvesterv1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    harvesterSystemNamespace,
			GenerateName: "harvester-iso-",
			Labels: map[string]string{
				harvesterUpgradeLabel: r.upgrade.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				upgradeReference(r.upgrade),
			},
		},
		Spec: harvesterv1.VirtualMachineImageSpec{
			DisplayName: getISODisplayNameImageName(r.upgrade.Name, r.upgrade.Spec.Version),
			SourceType:  v1beta1.VirtualMachineImageSourceTypeDownload,
			URL:         isoURL,
			Checksum:    checksum,
		},
	}

	return r.h.vmImageClient.Create(imageSpec)
}

func (r *UpgradeRepo) GetImage(imageName string) (*harvesterv1.VirtualMachineImage, error) {
	tokens := strings.Split(imageName, "/")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("Invalid image format %s", imageName)
	}

	image, err := r.h.vmImageCache.Get(tokens[0], tokens[1])
	if err != nil {
		return nil, err
	}
	return image, nil
}

func (r *UpgradeRepo) createVM(image *harvesterv1.VirtualMachineImage) (*kv1.VirtualMachine, error) {
	vmName := fmt.Sprintf("%s%s", repoVMNamePrefix, r.upgrade.Name)
	vmRun := true
	var bootOrder uint = 1
	evictionStrategy := kv1.EvictionStrategyLiveMigrate

	disk0Claim := fmt.Sprintf("%s-disk-0", vmName)
	volumeMode := corev1.PersistentVolumeBlock
	storageClassName := image.Status.StorageClassName
	pvcSpec := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: disk0Claim,
			Annotations: map[string]string{
				"harvesterhci.io/imageId": fmt.Sprintf("%s/%s", image.Namespace, image.Name),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("10Gi"),
				},
			},
			VolumeMode:       &volumeMode,
			StorageClassName: &storageClassName,
		},
	}
	pvc, err := json.Marshal([]corev1.PersistentVolumeClaim{pvcSpec})
	if err != nil {
		return nil, err
	}

	vm := kv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: upgradeNamespace,
			Labels: map[string]string{
				"harvesterhci.io/creator":      "harvester",
				harvesterUpgradeLabel:          r.upgrade.Name,
				harvesterUpgradeComponentLabel: upgradeComponentRepo,
			},
			Annotations: map[string]string{
				"harvesterhci.io/volumeClaimTemplates": string(pvc),
				"networks.harvesterhci.io/ips":         "[]",
				util.RemovedPVCsAnnotationKey:          disk0Claim,
			},
			OwnerReferences: []metav1.OwnerReference{
				upgradeReference(r.upgrade),
			},
		},
		Spec: kv1.VirtualMachineSpec{
			Running: &vmRun,
			Template: &kv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"harvesterhci.io/creator":      "harvester",
						"harvesterhci.io/vmName":       vmName,
						harvesterUpgradeLabel:          r.upgrade.Name,
						harvesterUpgradeComponentLabel: upgradeComponentRepo,
					},
				},
				Spec: kv1.VirtualMachineInstanceSpec{
					Domain: kv1.DomainSpec{
						CPU: &kv1.CPU{
							Cores:   1,
							Sockets: 1,
							Threads: 1,
						},
						Devices: kv1.Devices{
							Disks: []kv1.Disk{
								{
									BootOrder: &bootOrder,
									DiskDevice: kv1.DiskDevice{
										CDRom: &kv1.CDRomTarget{
											Bus: "sata",
										},
									},
									Name: "disk-0",
								},
								{
									DiskDevice: kv1.DiskDevice{
										CDRom: &kv1.CDRomTarget{
											Bus: "sata",
										},
									},
									Name: "cloudinitdisk",
								},
							},
							Inputs: []kv1.Input{
								{
									Bus:  "usb",
									Name: "tablet",
									Type: "tablet",
								},
							},
							Interfaces: []kv1.Interface{
								{
									InterfaceBindingMethod: kv1.InterfaceBindingMethod{
										Masquerade: &kv1.InterfaceMasquerade{},
									},
									Model: "virtio",
									Name:  "default",
								},
							},
						},
						Machine: &kv1.Machine{
							Type: "q35",
						},
						Resources: kv1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1G"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1G"),
							},
						},
					},
					EvictionStrategy: &evictionStrategy,
					Hostname:         vmName,
					Networks: []kv1.Network{
						{
							Name: "default",
							NetworkSource: kv1.NetworkSource{
								Pod: &kv1.PodNetwork{},
							},
						},
					},
					Volumes: []kv1.Volume{
						{
							Name: "disk-0",
							VolumeSource: kv1.VolumeSource{
								PersistentVolumeClaim: &kv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: disk0Claim,
									},
								},
							},
						},
						{
							Name: "cloudinitdisk",
							VolumeSource: kv1.VolumeSource{
								CloudInitNoCloud: &kv1.CloudInitNoCloudSource{
									UserData: repoVMUserData,
								},
							},
						},
					},
					ReadinessProbe: &kv1.Probe{
						Handler: kv1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/harvester-iso/harvester-release.yaml",
								Port: intstr.FromInt(80),
							},
						},
						TimeoutSeconds:   30,
						FailureThreshold: 5,
					},
				},
			},
		},
	}

	return r.h.vmClient.Create(&vm)
}

func (r *UpgradeRepo) getRepoServiceName() string {
	return fmt.Sprintf("%s%s", repoServiceNamePrefix, r.upgrade.Name)
}

func (r *UpgradeRepo) createService() (*corev1.Service, error) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: upgradeNamespace,
			Name:      r.getRepoServiceName(),
			OwnerReferences: []metav1.OwnerReference{
				upgradeReference(r.upgrade),
			},
			Labels: map[string]string{
				"harvesterhci.io/creator":      "harvester",
				harvesterUpgradeLabel:          r.upgrade.Name,
				harvesterUpgradeComponentLabel: upgradeComponentRepo,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				harvesterUpgradeLabel:          r.upgrade.Name,
				harvesterUpgradeComponentLabel: upgradeComponentRepo,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}

	return r.h.serviceClient.Create(&service)
}

func (r *UpgradeRepo) getInfo() (*UpgradeRepoInfo, error) {
	releaseURL := fmt.Sprintf("http://%s.%s/harvester-iso/harvester-release.yaml", r.getRepoServiceName(), upgradeNamespace)

	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpect status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info := UpgradeRepoInfo{}
	err = yaml.Unmarshal(body, &info.Release)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *UpgradeRepo) getInfoStr() (string, error) {
	info, err := r.getInfo()
	if err != nil {
		return "", err
	}
	out, err := info.Marshall()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
