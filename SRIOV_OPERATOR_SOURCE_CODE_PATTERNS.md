# SR-IOV Operator: Source Code Patterns to Investigate

**Purpose:** Specific code patterns and file locations to check in the sriov-network-operator GitHub repository

---

## Repository Structure (Expected)

```
sriov-network-operator/
├── cmd/
│   └── manager/
│       └── main.go                    ← Look here: Controller registration
├── controllers/
│   ├── sriovnetwork_controller.go     ← CRITICAL: Main reconciliation logic
│   ├── sriovnetworkpool_controller.go
│   ├── sriovoperatorconfig_controller.go
│   └── ...
├── api/
│   └── v1/
│       ├── sriovnetwork_types.go      ← SriovNetwork CRD definition
│       └── ...
├── pkg/
│   ├── webhook/                       ← Admission webhooks
│   ├── utils/
│   └── ...
└── config/
    └── rbac/
        ├── role.yaml                   ← RBAC permissions
        └── role_binding.yaml
```

---

## Critical File 1: main.go - Controller Registration

### Location
`cmd/manager/main.go` or `main.go` in root

### What to Look For: WORKING CODE

```go
package main

import (
    // ...
    sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
    "github.com/openshift/sriov-network-operator/controllers"
    // ...
)

func main() {
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        // ...
    })
    if err != nil {
        // ...
    }

    // ✅ THIS MUST EXIST
    if err = (&controllers.SriovNetworkReconciler{
        Client: mgr.GetClient(),
        Scheme: mgr.GetScheme(),
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "SriovNetwork")
        os.Exit(1)
    }

    // ...
    mgr.Start(ctrl.SetupSignalHandler())
}
```

### What to Look For: BROKEN CODE (BUG)

```go
// ❌ BUG: Controller not registered at all
// The SriovNetworkReconciler setup is completely missing

// ❌ BUG: Controller registration commented out
/*
if err = (&controllers.SriovNetworkReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "SriovNetwork")
    os.Exit(1)
}
*/

// ❌ BUG: Only watches SriovNetworkNodePolicy, not SriovNetwork
if err = (&controllers.SriovNetworkNodePolicyReconciler{
    // ...
}).SetupWithManager(mgr); err != nil {
    // ...
}
// Missing: SriovNetworkReconciler registration
```

### How to Verify
```bash
# Search for SriovNetworkReconciler in main.go
grep -n "SriovNetworkReconciler" cmd/manager/main.go

# Should find at least:
# - Import statement
# - SetupWithManager call
# - Error handling
```

---

## Critical File 2: sriovnetwork_controller.go - Reconciliation Logic

### Location
`controllers/sriovnetwork_controller.go`

### Expected Structure

```go
package controllers

import (
    "context"
    // ...
    sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
    nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
    // ...
)

// SriovNetworkReconciler reconciles a SriovNetwork object
type SriovNetworkReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete

func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    // ✅ MUST HAVE: This method
    // Reconciliation logic here
}

func (r *SriovNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // ✅ MUST HAVE: This method
    return ctrl.NewControllerManagedBy(mgr).
        For(&sriovnetworkv1.SriovNetwork{}).
        Complete(r)
}
```

### Working Reconcile Function Pattern

```go
func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // 1. Fetch the SriovNetwork resource
    sriovNet := &sriovnetworkv1.SriovNetwork{}
    if err := r.Get(ctx, req.NamespacedName, sriovNet); err != nil {
        if apierrors.IsNotFound(err) {
            // Object was deleted, clean up NAD
            return ctrl.Result{}, nil
        }
        log.Error(err, "unable to fetch SriovNetwork")
        return ctrl.Result{}, err
    }

    // 2. Create or update NetworkAttachmentDefinition
    nad := &nadv1.NetworkAttachmentDefinition{}
    nadName := client.ObjectKey{
        Name:      sriovNet.Name,
        Namespace: sriovNet.Spec.NetworkNamespace,
    }

    err := r.Get(ctx, nadName, nad)
    if err != nil && apierrors.IsNotFound(err) {
        // NAD doesn't exist, create it
        nad = r.constructNAD(sriovNet)
        if err := r.Create(ctx, nad); err != nil {
            log.Error(err, "failed to create NAD")
            // ✅ UPDATE STATUS
            sriovNet.Status.State = "Failed"
            sriovNet.Status.Error = fmt.Sprintf("failed to create NAD: %v", err)
            r.Status().Update(ctx, sriovNet)
            return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
        }
        log.Info("created NAD", "namespace", nadName.Namespace, "name", nadName.Name)
    } else if err != nil {
        log.Error(err, "error fetching NAD")
        return ctrl.Result{}, err
    }

    // 3. Update status to ready
    sriovNet.Status.State = "Ready"
    sriovNet.Status.Error = ""
    if err := r.Status().Update(ctx, sriovNet); err != nil {
        log.Error(err, "failed to update status")
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}

func (r *SriovNetworkReconciler) constructNAD(
    sriovNet *sriovnetworkv1.SriovNetwork,
) *nadv1.NetworkAttachmentDefinition {
    // Build the NetworkAttachmentDefinition
    cniConfig := r.buildCNIConfig(sriovNet)

    nad := &nadv1.NetworkAttachmentDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name:      sriovNet.Name,
            Namespace: sriovNet.Spec.NetworkNamespace,
        },
        Spec: nadv1.NetworkAttachmentDefinitionSpec{
            Config: cniConfig,
        },
    }

    return nad
}

func (r *SriovNetworkReconciler) buildCNIConfig(
    sriovNet *sriovnetworkv1.SriovNetwork,
) string {
    // Build SR-IOV CNI configuration
    return fmt.Sprintf(`{
        "cniVersion": "0.4.0",
        "name": "%s",
        "type": "sriov",
        ...
    }`, sriovNet.Name)
}
```

### Broken/Missing Pattern (BUG)

```go
// ❌ BUG: Reconcile function doesn't try to create NAD
func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    // Only fetches SriovNetwork but doesn't do anything with it
    sriovNet := &sriovnetworkv1.SriovNetwork{}
    if err := r.Get(ctx, req.NamespacedName, sriovNet); err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil  // ← Returns immediately without creating NAD
}

// ❌ BUG: Reconcile function has NAD creation but no error logging
func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    // ...
    if err := r.Create(ctx, nad); err != nil {
        // Silent failure - error not logged or status updated
        return ctrl.Result{}, nil  // ← Returns success despite error
    }
    // ...
}

// ❌ BUG: Status never updated
func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    // ... NAD creation succeeds
    // BUT never calls:
    // r.Status().Update(ctx, sriovNet)
    // → User has no visibility into what happened
}
```

### How to Verify
```bash
# Search for the Reconcile method
grep -n "func.*Reconcile" controllers/sriovnetwork_controller.go

# Search for NAD creation attempts
grep -n "NetworkAttachmentDefinition\|NAD\|nadv1" controllers/sriovnetwork_controller.go

# Search for Status updates
grep -n "\.Status()" controllers/sriovnetwork_controller.go

# Count error logging
grep -c "log\.Error" controllers/sriovnetwork_controller.go
```

---

## Critical File 3: Admission Webhook

### Location
`pkg/webhook/` directory

### Expected Structure

```go
package webhook

import (
    // ...
    sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
)

// +kubebuilder:webhook:path=/validate-sriovnetwork-openshift-io-v1-sriovnetwork,mutating=false,
// failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1,
// clientConfig={}
// +kubebuilder:object:generate=false

type SriovNetworkValidator struct{}

func (v *SriovNetworkValidator) ValidateSriovNetwork(
    ctx context.Context,
    sriovNet *sriovnetworkv1.SriovNetwork,
) error {
    // Validation logic
    return nil
}
```

### Problematic Pattern (BUG)

```go
// ❌ BUG: failurePolicy=Ignore allows silent failures
// +kubebuilder:webhook:path=/validate...,failurePolicy=ignore,...

// This allows the webhook to fail and let the request through anyway
// But the controller might not see the event

// ✅ BETTER: failurePolicy=fail
// +kubebuilder:webhook:path=/validate...,failurePolicy=fail,...

// This ensures the user sees the error immediately
```

### How to Verify
```bash
# Check webhook failure policy
kubectl get validatingwebhookconfiguration -A -o yaml | grep -A 10 "sriov"
# Look for: failurePolicy: Ignore vs failurePolicy: Fail

# Get webhook configuration
oc describe validatingwebhookconfiguration sriov-network-operator
```

---

## Critical File 4: RBAC Configuration

### Location
`config/rbac/role.yaml` or `config/rbac/role_binding.yaml`

### What to Look For: COMPLETE RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sriov-network-operator
rules:
# ✅ MUST HAVE: Permissions for SriovNetwork
- apiGroups:
  - sriovnetwork.openshift.io
  resources:
  - sriovnetworks
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
# ✅ MUST HAVE: Permissions for SriovNetwork status
- apiGroups:
  - sriovnetwork.openshift.io
  resources:
  - sriovnetworks/status
  verbs:
  - get
  - patch
  - update
# ✅ MUST HAVE: Permissions for NAD creation
- apiGroups:
  - k8s.cni.cncf.io
  resources:
  - network-attachment-definitions
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
```

### Broken Pattern (BUG)

```yaml
# ❌ BUG: Missing NAD permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sriov-network-operator
rules:
# Has permissions for SriovNetwork
- apiGroups:
  - sriovnetwork.openshift.io
  resources:
  - sriovnetworks
  verbs:
  - get
  - list
  - watch
  - update
  - patch
# ❌ MISSING: Permissions for NAD creation!
# No k8s.cni.cncf.io rules
```

### How to Verify
```bash
# Get operator's ClusterRole
oc get clusterrole sriov-network-operator -o yaml

# Search for network-attachment-definitions
oc get clusterrole sriov-network-operator -o yaml | grep -A 5 "network-attachment"

# Test if operator can create NADs
oc auth can-i create network-attachment-definitions \
  --as=system:serviceaccount:openshift-sriov-network-operator:default \
  --all-namespaces
```

---

## Critical File 5: SriovNetwork CRD Definition

### Location
`api/v1/sriovnetwork_types.go`

### Expected Structure

```go
package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SriovNetworkSpec defines the desired state of SriovNetwork
type SriovNetworkSpec struct {
    // NetworkNamespace is the target namespace where the NAD will be created
    NetworkNamespace string `json:"networkNamespace,omitempty"`
    
    // ResourceName specifies the resource name defined in SriovNetworkNodePolicy
    ResourceName string `json:"resourceName,omitempty"`
    
    // IPAM is the IP Address Management configuration
    IPAM string `json:"ipam,omitempty"`
    
    // Other fields...
}

// SriovNetworkStatus defines the observed state of SriovNetwork
type SriovNetworkStatus struct {
    // State can be "Ready", "Failed", "Pending"
    State string `json:"state,omitempty"`
    
    // Error message if status is Failed
    Error string `json:"error,omitempty"`
    
    // Conditions for detailed status
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`

type SriovNetwork struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              SriovNetworkSpec   `json:"spec,omitempty"`
    Status            SriovNetworkStatus `json:"status,omitempty"`
}
```

### Bug Pattern (CRD Definition)

```go
// ❌ BUG: Status is not defined or not subresourced
type SriovNetwork struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              SriovNetworkSpec `json:"spec,omitempty"`
    // Missing: Status field
}

// ❌ BUG: Missing subresource tag
// Missing: +kubebuilder:subresource:status

// This means the CRD won't support status updates via r.Status().Update()
```

### How to Verify
```bash
# Check CRD definition
oc get crd sriovnetworks.sriovnetwork.openshift.io -o yaml

# Look for status subresource
oc get crd sriovnetworks.sriovnetwork.openshift.io -o yaml | grep -A 5 "subresources"

# Check if status updates work
oc patch sriovnetwork test -n default --type merge --subresource status \
  -p '{"status":{"state":"Ready"}}'
```

---

## Search Commands for Quick Investigation

### Find all Reconcile methods
```bash
find . -name "*.go" -type f -exec grep -l "func.*Reconcile" {} \;
```

### Find all controller registrations
```bash
grep -r "SetupWithManager" --include="*.go" .
```

### Find all NAD creation code
```bash
grep -r "NetworkAttachmentDefinition\|nadv1" --include="*.go" controllers/
```

### Find all status updates
```bash
grep -r "\.Status()" --include="*.go" controllers/
```

### Find webhook configurations
```bash
find . -path "*webhook*" -name "*.go" -type f
```

### Find RBAC rules
```bash
find config -name "*role*.yaml" -type f
```

---

## Expected vs Actual File Structure

### EXPECTED (Working operator)
```
controllers/
├── sriovnetwork_controller.go           ← Handles SriovNetwork CRs
├── sriovnetworkpool_controller.go
├── sriovnetworkpolicy_controller.go
├── sriovnetworknodestate_controller.go
└── suite_test.go
```

### ACTUAL (Broken operator - BUG)
```
controllers/
├── sriovnetworkpool_controller.go
├── sriovnetworkpolicy_controller.go     ← Only handles policy
├── sriovnetworknodestate_controller.go
└── suite_test.go
# ❌ MISSING: sriovnetwork_controller.go
```

---

## Code Investigation Checklist

- [ ] sriovnetwork_controller.go EXISTS
- [ ] SriovNetworkReconciler type is DEFINED
- [ ] Reconcile() method is IMPLEMENTED
- [ ] SetupWithManager() method is IMPLEMENTED
- [ ] SetupWithManager() calls For(&sriovnetworkv1.SriovNetwork{})
- [ ] main.go REGISTERS SriovNetworkReconciler
- [ ] Reconcile() attempts to CREATE NAD
- [ ] Reconcile() LOGS errors
- [ ] Reconcile() UPDATES SriovNetwork status
- [ ] RBAC includes k8s.cni.cncf.io network-attachment-definitions
- [ ] CRD includes Status subresource
- [ ] Admission webhook has proper failurePolicy
- [ ] No obvious return early statements before NAD creation

---

## Filing a Bug Report to Maintainers

Create a GitHub issue with:

```markdown
## Issue: SriovNetwork objects not reconciled

### Description
When a SriovNetwork CR is created, the operator doesn't process it.

### Evidence
1. SriovNetworkReconciler file exists: [YES/NO]
2. Reconcile method creates NAD: [YES/NO]
3. Operator logs show reconciliation: [YES/NO]
4. NAD appears in target namespace: [YES/NO]
5. Operator version: [version]

### Operator Logs
[Include full operator logs showing SriovNetwork creation attempt]

### Files to review:
- `controllers/sriovnetwork_controller.go` - Reconciliation logic
- `cmd/manager/main.go` - Controller registration
- `api/v1/sriovnetwork_types.go` - CRD definition
- `config/rbac/role.yaml` - RBAC permissions
```

