#!/bin/bash -ex

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
HOST_DIR="${HOST_DIR:-/host}"
UPGRADE_TMP_DIR=$HOST_DIR/usr/local/upgrade_tmp

source $SCRIPT_DIR/lib.sh

# Create a systemd service on host to reboot the host if this running pod succeeds.
# This prevents job become from entering `Error`.
reboot_if_job_succeed()
{
  cat > $HOST_DIR/tmp/upgrade-reboot.sh << EOF
#!/bin/bash -ex
HARVESTER_UPGRADE_POD_NAME=$HARVESTER_UPGRADE_POD_NAME

EOF

  cat >> $HOST_DIR/tmp/upgrade-reboot.sh << 'EOF'
source /etc/bash.bashrc.local
pod_id=$(crictl pods --name $HARVESTER_UPGRADE_POD_NAME --namespace harvester-system -o json | jq -er '.items[0].id')

# get `upgrade` container ID
container_id=$(crictl ps --pod $pod_id --name apply -o json -a | jq -er '.containers[0].id')
container_state=$(crictl inspect $container_id | jq -er '.status.state')

if [ "$container_state" = "CONTAINER_EXITED" ]; then
  container_exit_code=$(crictl inspect $container_id | jq -r '.status.exitCode')

  if [ "$container_exit_code" = "0" ]; then
    sleep 10
    reboot
    exit 0
  fi
fi

exit 1
EOF

  chmod +x $HOST_DIR/tmp/upgrade-reboot.sh

  cat > $HOST_DIR/run/systemd/system/upgrade-reboot.service << 'EOF'
[Unit]
Description=Upgrade reboot
[Service]
Type=simple
ExecStart=/tmp/upgrade-reboot.sh
Restart=always
RestartSec=10
EOF

  chroot $HOST_DIR systemctl daemon-reload
  chroot $HOST_DIR systemctl start upgrade-reboot
}

preload_images()
{
  export CONTAINER_RUNTIME_ENDPOINT=unix:///$HOST_DIR/run/k3s/containerd/containerd.sock
  export CONTAINERD_ADDRESS=$HOST_DIR/run/k3s/containerd/containerd.sock

  CTR="$HOST_DIR/$(readlink $HOST_DIR/var/lib/rancher/rke2/bin)/ctr"
  if [ -z "$CTR" ];then
    echo "Fail to get host ctr binary."
    exit 1
  fi

  metadata=$(mktemp --suffix=.yaml)
  curl -fL $UPGRADE_REPO_BUNDLE_METADATA -o $metadata

  tmp_image_archives=$(mktemp -d -p $UPGRADE_TMP_DIR)

  # Common container images. Load with containerd.
  yq -e -o=json e '.images.common' $metadata | jq -r '.[] | [.list, .archive] | @tsv' |
    while IFS=$'\t' read -r list archive; do
      archive_name=$(basename -s .tar.zst $archive)
      image_list_url="$UPGRADE_REPO_BUNDLE_ROOT/$list"
      archive_url="$UPGRADE_REPO_BUNDLE_ROOT/$archive"
      image_list_file="${tmp_image_archives}/$(basename $list)"
      archive_file="${tmp_image_archives}/${archive_name}.tar"

      # Check if images already exist
      curl -sfL $image_list_url | sort > $image_list_file
      missing=$($CTR -n k8s.io images ls -q | grep -v ^sha256 | sort | comm -23 $image_list_file -)
      if [ -z "$missing" ]; then
        echo "Images in $image_list_file already present in the system. Skip preloading."
        continue
      fi

      curl -sfL $archive_url | zstd -d -f --no-progress -o $archive_file
      $CTR -n k8s.io image import $archive_file
      rm -f $archive_file
    done

  rm -rf $tmp_image_archives

  download_image_archives_from_repo "rke2" $HOST_DIR/var/lib/rancher/rke2/agent/images
  download_image_archives_from_repo "agent" $HOST_DIR/var/lib/rancher/agent/images
}

get_running_vm_count()
{
  local count

  count=$(kubectl get vmi -A -l kubevirt.io/nodeName=$HARVESTER_UPGRADE_NODE_NAME -ojson | jq '.items | length' || true)
  echo $count
}

wait_vms_out()
{
  vm_count="$(get_running_vm_count)"
  until [ "$vm_count" = "0" ]
  do
    echo "Waiting for VM live-migration or shutdown...($vm_count left)"
    sleep 5
    vm_count="$(get_running_vm_count)"
  done
}

shutdown_repo_vm() {
  # We don't need to live-migrate upgrade repo VM. Just make sure it's up when we need it.
  # Shutdown it if it's running on this upgrading node.
  repo_vm_name="upgrade-repo-$HARVESTER_UPGRADE_NAME"
  repo_node=$(kubectl get vmi -n harvester-system $repo_vm_name -o yaml | yq -e e '.status.nodeName' -)
  if [ "$repo_node" = "$HARVESTER_UPGRADE_NODE_NAME" ]; then
  	echo "Stop upgrade repo VM: $repo_vm_name"
  	virtctl stop $repo_vm_name -n harvester-system
  fi
}

shutdown_all_vms()
{
  kubectl get vmi -A -o json |
    jq -r '.items[] | [.metadata.name, .metadata.namespace] | @tsv' |
    while IFS=$'\t' read -r name namespace; do
      if [ -z "$name" ]; then
        break
      fi
      echo "Stop ${namespace}/${name}"
      virtctl stop $name -n $namespace
    done
}

shutdown_non_migrate_able_vms()
{
  # VMs with nodeSelector
  kubectl get vmi -A -l kubevirt.io/nodeName=$HARVESTER_UPGRADE_NODE_NAME -o json |
    jq -r '.items[] | select(.spec.nodeSelector != null) | [.metadata.name, .metadata.namespace] | @tsv' |
    while IFS=$'\t' read -r name namespace; do
      if [ -z "$name" ]; then
        break
      fi
      echo "Stop ${namespace}/${name}"
      virtctl stop $name -n $namespace
    done

  # VMs with nodeAffinity
  kubectl get vmi -A -l kubevirt.io/nodeName=$HARVESTER_UPGRADE_NODE_NAME -o json |
    jq -r '.items[] | select(.spec.affinity.nodeAffinity != null) | [.metadata.name, .metadata.namespace] | @tsv' |
    while IFS=$'\t' read -r name namespace; do
      if [ -z "$name" ]; then
        break
      fi
      echo "Stop ${namespace}/${name}"
      virtctl stop $name -n $namespace
    done
}

command_prepare()
{
  wait_repo
  detect_repo
  preload_images
}

wait_evacuation_pdb_gone()
{
  # TODO: fine-tune this to per-VM check
  until ! kubectl get pdb -o name -A | grep kubevirt-migration-pdb-kubevirt-evacuation-
  do
    echo "Waiting for evacuation PDB gone..."
    sleep 5
  done
}


command_pre_drain() {
  shutdown_non_migrate_able_vms

  # Live migrate VMs
  kubectl taint node $HARVESTER_UPGRADE_NODE_NAME --overwrite kubevirt.io/drain=draining:NoSchedule

  # Wait for VM migrated
  wait_vms_out

  # KubeVirt's pdb might cause drain fail
  wait_evacuation_pdb_gone
}

get_node_rke2_version() {
  kubectl get node $HARVESTER_UPGRADE_NODE_NAME -o yaml | yq -e e '.status.nodeInfo.kubeletVersion' -
}

upgrade_rke2() {
  patch_file=$(mktemp -p $UPGRADE_TMP_DIR)

cat > $patch_file <<EOF
spec:
  kubernetesVersion: $REPO_RKE2_VERSION
  rkeConfig: {}
EOF

  kubectl patch clusters.provisioning.cattle.io local -n fleet-local --patch-file $patch_file --type merge
}

wait_rke2_upgrade() {
  until [ "$(get_node_rke2_version)" = "$REPO_RKE2_VERSION" ]
  do
    echo "Waiting for RKE2 to be upgraded..."
    sleep 5
  done
}

rebrand_grub () {
  mount -o remount,rw $HOST_DIR/run/initramfs/cos-state/
  chroot $HOST_DIR grub2-editenv /run/initramfs/cos-state/grub_oem_env set "default_menu_entry=$REPO_OS_PRETTY_NAME"
  mount -o remount,ro $HOST_DIR/host/run/initramfs/cos-state/
}

upgrade_os() {
  CURRENT_OS_VERSION=$(source $HOST_DIR/etc/os-release && echo $PRETTY_NAME)

  if [ "$REPO_OS_PRETTY_NAME" = "$CURRENT_OS_VERSION" ]; then
    echo "Skip upgrading OS. The OS version is already \"$CURRENT_OS_VERSION\"."
    return
  fi
  
  # upgrade OS image and reboot
  mount --rbind $HOST_DIR/dev /dev
  mount --rbind $HOST_DIR/run /run

  if [ -n "$NEW_OS_SQUASHFS_IMAGE_FILE" ]; then
    tmp_rootfs_squashfs="$NEW_OS_SQUASHFS_IMAGE_FILE"
  else
    tmp_rootfs_squashfs=$(mktemp -p $UPGRADE_TMP_DIR)
    curl -fL $UPGRADE_REPO_SQUASHFS_IMAGE -o $tmp_rootfs_squashfs
  fi

  tmp_rootfs_mount=$(mktemp -d) 
  mount $tmp_rootfs_squashfs $tmp_rootfs_mount

  bash -x $HOST_DIR/usr/sbin/cos upgrade --directory $tmp_rootfs_mount
  umount $tmp_rootfs_mount
  rm -rf $tmp_rootfs_squashfs

  umount -R /run

  # https://github.com/rancher-sandbox/cOS-toolkit/issues/928
  rebrand_grub || true

  reboot_if_job_succeed
}

start_repo_vm() {
  repo_vm=$(kubectl get vm -l "harvesterhci.io/upgrade=$HARVESTER_UPGRADE_NAME" -n harvester-system -o yaml | yq -e e '.items[0].metadata.name' -)
  if [ -z $repo_vm ]; then
    echo "Fail to get upgrade repo VM name."
    exit 1
  fi

  virtctl start $repo_vm -n harvester-system || true
}

command_post_drain() {
  wait_repo
  detect_repo

  # A post-drain signal from Rancher doesn't mean RKE2 agent/server is already patched and restarted
  # Let's wait until the RKE2 settled.
  wait_rke2_upgrade

  kubectl taint node $HARVESTER_UPGRADE_NODE_NAME kubevirt.io/drain- || true
  upgrade_os
}

command_single_node_upgrade() {
  echo "Upgrade single node"

  wait_repo
  detect_repo

  # Copy OS things, we need to shutdown repo VMs.
  NEW_OS_SQUASHFS_IMAGE_FILE=$(mktemp -p $UPGRADE_TMP_DIR)
  curl -fL $UPGRADE_REPO_SQUASHFS_IMAGE -o $NEW_OS_SQUASHFS_IMAGE_FILE

  # Stop all VMs
  shutdown_all_vms
  wait_vms_out

  # Upgarde RKE2
  upgrade_rke2
  wait_rke2_upgrade

  # Upgrade OS
  upgrade_os
}

mkdir -p $UPGRADE_TMP_DIR

case $1 in
  prepare)
    command_prepare
    ;;
  pre-drain)
    command_pre_drain
    ;;
  post-drain)
    command_post_drain
    ;;
  single-node-upgrade)
    command_single_node_upgrade
    ;;
esac
