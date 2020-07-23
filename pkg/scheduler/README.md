### How to build and deploy scheduler extender

 1. Build binary and push image (if you don't change extender' code you don't have to rebuild
 binary and image):
    `make all`
    Image `10.244.120.194:9042/scheduler-extender:0.0.1` that is built has already present
    in the registry

 2. On node where kube scheduler pod is run create folder `/etc/kubernetes/scheduler`
 e.g. 
 ```
 ssh root@provo-goop mkdir -p /etc/kubernetes/scheduler
```
 3. Copy files `config.yaml` and `policy.yaml` from `deploy` folder to the `/etc/kubernetes/scheduler`
 on the node.
 e.g. 
 ```
 scp deploy/policy.yaml deploy/config.yaml root@provo-goop:/etc/kubernetes/scheduler
 ```
 4. Apply extender manifest:
    `kubectl apply -f deploy/extender.yaml`
 5. Modify kube-scheduler manifest on the node. Config file is located in `/etc/kubernetes/manifests/kube-scheduler.yaml`
    
    - add next volumes in `.spec`:
    ```
    - name: scheduler-config
      hostPath:
        path: /etc/kubernetes/scheduler/config.yaml
        type: File
    - name: scheduler-policy
      hostPath:
        path: /etc/kubernetes/scheduler/policy.yaml
        type: File
    ```
    - add volume mounts in `.spec.containers[0].volumeMounts`:
    ```
    - mountPath: /etc/kubernetes/scheduler/config.yaml
      name: scheduler-config
      readOnly: true
    - mountPath: /etc/kubernetes/scheduler/policy.yaml
      name: scheduler-policy
      readOnly: true
    ```
    - add next params for kube-scheduler entrypoint in `.spec.containers[0].command`:
    ```
    - --config=/etc/kubernetes/scheduler/config.yaml
    ```
    After you save changes in `kube-scheduler.yaml` kubernetes will restart scheduler.
    
 6. Apply some pod manifest
 7. Run `kubectl logs -f csi-baremetal-se-0 -n kube-system` and observe as scheduler extender works
 
## Limitation
    Since it is POC implementation extender does not check storage class and size of volumes
    and just checks that amount of ACs on node is >= amount of volumes to be provisioned.