# Context and Usage Guide

**Status**: ✅ COMPLETE CONTEXT & USAGE  
**Date**: November 12, 2025

---

## When Does SR-IOV Operator Create NAD?

### The Trigger

SR-IOV operator creates a NetworkAttachmentDefinition (NAD) when you **create a SriovNetwork custom resource**:

```yaml
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetwork
metadata:
  name: my-network
  namespace: openshift-sriov-network-operator
spec:
  networkNamespace: user-namespace    # Where NAD will be created
  resourceName: device-name            # Device type
  ipam: {"type": "static"}
```

**When you create this:**
1. Operator controller detects new SriovNetwork
2. Operator's reconciler calls `RenderNetAttDef()`
3. NAD is generated (with incomplete config - THE BUG)
4. NAD is created in specified namespace

---

## Why Normal Operation Works Fine

### Scenario 1: Static Pre-configured Networks
```
Production Environment:
  ✓ Networks pre-defined during cluster setup
  ✓ NADs created long ago or manually
  ✓ Operator just maintains them
  ✓ No new NAD generation needed
  ✓ Bug doesn't manifest
```

### Scenario 2: New Network Creation (Bug Manifests)
```
Your Testing Scenario:
  ✓ Create NEW SriovNetwork resource
  ✓ Operator MUST generate new NAD
  ❌ NAD generated with incomplete config
  ✓ Create pod with network annotation
  ❌ CNI plugin reads incomplete NAD
  ❌ Pod attachment fails
```

### Scenario 3: Operator Restart (Bug Manifests)
```
Production After Operator Crash:
  ✓ Operator running (NADs exist)
  ✓ Operator crashes/restarts
  ✓ NADs might be deleted
  ✓ Operator recreates them
  ❌ Recreated NADs have incomplete config
  ❌ Pods fail to attach
  ✓ This is when bug appears in production
```

---

## Complete Lifecycle Flow

```
Phase 1: Setup
──────────────
1. Install SR-IOV Operator
   └─ Just deployment/daemonset setup
   └─ No NAD creation yet

2. Create SriovNetworkNodePolicy
   ├─ Tells operator which NICs to enable
   ├─ Node daemon creates VFs
   ├─ Kernel updates applied
   └─ Still no NAD creation yet!

Phase 2: Network Definition (BUG TRIGGERED HERE)
─────────────────────────────────────────────────
3. Create SriovNetwork
   ├─ User: "I want a network called 'my-net'"
   ├─ Operator reconciler triggered
   ├─ Calls RenderNetAttDef()
   ├─ Generates NAD
   ├─ ❌ BUG: NAD incomplete
   └─ NAD created with missing resourceName in spec.config

Phase 3: Pod Usage (BUG MANIFESTS HERE)
───────────────────────────────────────
4. Create Pod requesting network
   ├─ Pod spec: networks: [{name: my-net}]
   ├─ Kubernetes schedules to SR-IOV node
   ├─ Kubelet calls CNI plugin
   ├─ Multus plugin reads NAD
   ├─ Passes to SR-IOV CNI plugin
   ├─ SR-IOV CNI reads spec.config
   ├─ ❌ Can't find resourceName
   ├─ ❌ Can't find pciAddress
   ├─ ERROR: "VF pci addr is required"
   ├─ Pod sandbox creation fails
   └─ Pod stays in Pending state
```

---

## Why Your Tests Expose The Bug

Your tests are comprehensive:

```
Test Structure:
  1. ✅ Install operator
  2. ✅ Create policies (enable VFs on NICs)
  3. ✅ Create NETWORKS (TRIGGERS NAD GENERATION)
  4. ✅ Try to USE networks (EXPOSES BUG)
  5. ✅ Uninstall operator
  6. ✅ Reinstall operator
  7. ✅ Repeat steps 3-4 (TESTS LIFECYCLE)

Why Production Doesn't Catch It:
  - Uses pre-configured networks
  - Doesn't create new networks frequently
  - Doesn't test operator restart
  - Doesn't check NAD completeness
```

---

## Pod Attachment Deep Dive

### What CNI Plugin Needs

When SR-IOV CNI plugin receives network config, it needs:

```json
{
  "cniVersion": "0.4.0",           // Protocol version
  "name": "network-name",           // Network name
  "type": "sriov",                  // CNI type
  "resourceName": "device-id",      // Device resource identifier ← CRITICAL
  "pciAddress": "0000:02:01.2",     // Device PCI address ← CRITICAL
  "vlan": 0,                        // VLAN tag
  "ipam": {"type": "static"}        // IP allocation mode
}
```

### Why These Fields Matter

#### resourceName
- **Purpose**: Identify which device resource to use
- **Used By**: Kubelet to request device from device plugin
- **Impact If Missing**: CNI can't find device → attachment fails

#### pciAddress
- **Purpose**: Identify specific Virtual Function (VF) on the node
- **Used By**: SR-IOV driver to attach VF to container
- **Impact If Missing**: CNI can't attach VF → attachment fails

### What Actually Happens

```
CNI Plugin receives:
{
  "cniVersion": "1.0.0",
  "name": "network-name",
  "type": "sriov",
  "vlan": 0,
  "logLevel": "info",
  "ipam": {"type": "static"}
}

CNI tries to load config:
  1. Looks for "resourceName" → ❌ NOT FOUND
  2. Looks for "pciAddress" → ❌ NOT FOUND
  3. Tries to continue → ❌ CANNOT PROCEED
  4. Throws error: "VF pci addr is required"
  5. Pod attachment FAILS
```

---

## Bug Timeline

```
Normal Cluster Operation:
  T0: Install operator
  T0+: Networks pre-configured
  T0+: Pods attach fine (using pre-configured NADs)
  ✅ Bug hidden - no new NAD creation

Your Test Run:
  T0: Install operator
  T1: Create SriovNetworkNodePolicy
  T2: Create NEW SriovNetwork ← NAD generated with incomplete config
  T3: Create pod ← Tries to use network
  T4: Pod attachment fails ← BUG EXPOSED!

After Operator Restart (Production):
  T0: Operator running (NADs exist)
  T1: Operator crashes/restarts
  T2: Existing NADs might be lost/regenerated
  T3: New NADs have incomplete config
  T4: Pods fail to attach ← BUG APPEARS IN PRODUCTION!
```

---

## Using This Package

### Path 1: Quick Understanding (15 minutes)
```
1. Read: 1_BUG_INVESTIGATION_SUMMARY.md
2. Read: 6_CONTEXT_AND_USAGE_GUIDE.md (this file)
3. Read: 5_DOCUMENTATION_SUMMARY.md
→ You understand when/why/how bug manifests
```

### Path 2: Technical Deep Dive (45 minutes)
```
1. Do Path 1 (15 min)
2. Read: 2_ROOT_CAUSE_AND_CODE_ANALYSIS.md (15 min)
3. Read: 3_BUG_EVIDENCE_AND_REPRODUCTION.md (15 min)
→ You have complete technical understanding
```

### Path 3: File Upstream (20 minutes)
```
1. Read: 4_UPSTREAM_BUG_REPORT.md
2. Prepare: reproduce_incomplete_nad_bug.sh
3. Attach: bug_evidence/ directory logs
4. Submit with report
→ Ready for upstream filing
```

---

## Key Usage Scenarios

### Scenario A: Operator Team Troubleshooting
```
Question: "Why do pods fail to attach after operator restart?"
Answer: Check if NAD has resourceName in spec.config
Location: This package → Path 2
```

### Scenario B: QA Testing SR-IOV
```
Question: "How do we test SR-IOV operator comprehensively?"
Answer: Follow your test pattern (create new networks, attach pods)
Location: This package → 6_CONTEXT_AND_USAGE_GUIDE.md
```

### Scenario C: Upstream Bug Filing
```
Question: "How do we report this to SR-IOV operator team?"
Answer: Use the prepared bug report
Location: This package → 4_UPSTREAM_BUG_REPORT.md
```

### Scenario D: Production Deployment
```
Question: "Can we use SR-IOV networking safely?"
Answer: Yes, if using pre-configured networks
Impact: Bug only manifests with dynamic network creation
```

---

## Best Practices

### To Avoid This Bug in Deployments
- ✅ Pre-configure SR-IOV networks
- ✅ Use StatefulSet with stable network definitions
- ✅ Avoid dynamic network creation
- ✅ Test operator restart scenarios in staging

### To Catch This Bug in Testing
- ✅ Create new networks during tests
- ✅ Actually use networks (attach pods)
- ✅ Test operator lifecycle (restart, reinstall)
- ✅ Verify NAD completeness

### To Report This Bug Effectively
- ✅ Include operator logs
- ✅ Show exact NAD output
- ✅ Provide reproduction script
- ✅ Specify when bug manifests

---

## Reference Table

| Aspect | Normal Ops | Your Tests | Manifestation |
|--------|-----------|-----------|---|
| Network Type | Pre-configured | Dynamic | Bug shows with dynamic |
| NAD Creation | Never (exists) | New | Bug only on creation |
| Pod Attachment | Works | Fails | Failure triggers exposure |
| Operator Restart | NAD preserved | NAD recreated | Bug on recreation |
| Detection | Missed | Caught | Testing matters! |

---

## Summary

- **When**: Operator creates NAD (SriovNetwork resource creation)
- **Why Normal Ops Work**: Pre-configured networks don't trigger NAD creation
- **Why Tests Catch It**: Dynamic network creation triggers bug
- **When Bug Shows**: Pod attachment with incomplete NAD config
- **How to Use This**: Choose path above based on your needs

---

*Complete context and usage guide. Everything you need to understand, troubleshoot, and file this bug.*

