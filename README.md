# Bsc thesis project
The goal of the project is to create a program that throttles Pods in a Kubernetes cluster according to some rules.

# K3s installation
1. `$ curl -sfL https://get.k3s.io | sh -`
2. `$ k3s kubectl apply -f k3s/config.yml`
3. `$ k3s kubectl get pods,svc,ingress -n thesis-ns`

K3s doesn't currently support cgroup v2 ([related discussion](https://github.com/k3s-io/k3s/issues/900)). Version v1.20.3+k3s1 (due by February 17, 2021) is supposed to include the fixes. For the time being, the repo includes the  `k3s/k3s` executable (Linux) that I've built. Run with
`$ sudo k3s/k3s server`

# Enabling cgroup v2
1. `$ sudo gedit /etc/default/grub`
2. Append `cgroup_no_v1=net_prio,net_cls systemd.unified_cgroup_hierarchy=1` to the end of both `GRUB_CMDLINE_LINUX_DEFAULT` and `GRUB_CMDLINE_LINUX` lines.
3. Save file and exit gedit
4. `$ sudo update-grub`
5. Reboot your machine

# Dependencies:
Install with  
`$ sudo pacman -S base-devel go clang bpf libbpf`

# Usage
`$ cd ebpf_loader && go build -o podmgr . && ./podmgr`

# Todos
- Add ingress option (currently only egresses are supported)
- Add optino to load custom ebpf programs
- Add option to specify which container to include/exclude
- Add more rate limiting ebpf programs
